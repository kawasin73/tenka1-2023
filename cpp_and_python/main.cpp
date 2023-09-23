#include <iostream>
#include <vector>
#include <string>
#include <set>
#include <random>

using namespace std;

mt19937 mt;

const int N = 5;
const int Dj[] {+1, 0, -1, 0};
const int Dk[] {0, +1, 0, -1};
const int TOTAL_TURN = 294;

struct Agent {
    int i, j, k, d;
};

struct Cell {
    int owner, val;
};

struct GameLogic {
    vector<Cell> field;
    vector<Agent> agents;
    int turn;
    vector<int> move;
    vector<int> score;
    vector<int> area;
    vector<int> special;

    Cell get_cell(int i, int j, int k) {
        return field[field_idx(i, j, k)];
    }

    // move_list に従ってゲームを進行する
    void progress(int member_id, const vector<int>& move_list) {
        if (move_list.size() % 6 != 0) {
            cerr << "invalid move_list length" << endl;
            throw 1;
        }
        unsigned char counter[6 * N * N] = {};
        int fis[6] = {};
        for (unsigned i = 0; i < move_list.size(); i += 6) {
            // エージェントの移動処理
            for (int idx = 0; idx < 6; ++ idx){
                move[idx] = move_list[i + func1(member_id, idx)];
                if (move[idx] == -1 or move[idx] >= 4) {
                    continue;
                }
                rotate_agent(idx, move[idx]);
                move_forward(idx);
                int ii = agents[idx].i;
                int jj = agents[idx].j;
                int kk = agents[idx].k;
                fis[idx] = field_idx(ii, jj, kk);
                counter[fis[idx]] |= 1 << idx;
            }

            // フィールドの更新処理 (通常移動)
            for (int idx = 0; idx < 6; ++ idx) {
                if (move[idx] == -1 or move[idx] >= 4) {
                    continue;
                }
                int owner_id = idx < 3 ? idx : 5 - idx;
                if (check_counter(counter[fis[idx]], owner_id, idx) or field[fis[idx]].owner == owner_id) {
                    paint(owner_id, fis[idx]);
                }
            }

            for (int idx = 0; idx < 6; ++ idx) {
                if (move[idx] == -1 or move[idx] >= 4) {
                    continue;
                }
                counter[fis[idx]] = 0;
            }

            // フィールドの更新処理 (特殊移動)
            set<int> special_fis;
            for (int idx = 0; idx < 6; ++ idx) {
                if (move[idx] <= 3) {
                    continue;
                }
                special[idx] -= 1;
                int owner_id = idx < 3 ? idx : 5 - idx;
                if (move[idx] <= 7) {
                    // 5 マス前進
                    rotate_agent(idx, move[idx]);
                    for (int p = 0; p < 5; ++ p) {
                        move_forward(idx);
                        int ii = agents[idx].i;
                        int jj = agents[idx].j;
                        int kk = agents[idx].k;
                        int fi = field_idx(ii, jj, kk);
                        special_fis.insert(fi);
                        counter[fi] |= 1 << owner_id;
                    }
                } else {
                    // 指定したマスに移動
                    int m = move[idx] - 8;
                    int mi = func1(owner_id, m / 25);
                    int mj = m / 5 % 5;
                    int mk = m % 5;
                    {
                        int fi = field_idx(mi, mj, mk);
                        special_fis.insert(fi);
                        counter[fi] |= 1 << owner_id;
                    }
                    for (int d = 0; d < 4; ++ d) {
                        agents[idx].i = mi;
                        agents[idx].j = mj;
                        agents[idx].k = mk;
                        agents[idx].d = d;
                        move_forward(idx);
                        int ii = agents[idx].i;
                        int jj = agents[idx].j;
                        int kk = agents[idx].k;
                        int fi = field_idx(ii, jj, kk);
                        special_fis.insert(fi);
                        counter[fi] |= 1 << owner_id;
                    }
                    agents[idx].i = mi;
                    agents[idx].j = mj;
                    agents[idx].k = mk;
                    agents[idx].d = 0;
                }
            }

            for (int fi : special_fis) {
                switch (counter[fi]) {
                    case 1:
                        force_paint(0, fi);
                        break;
                    case 2:
                        force_paint(1, fi);
                        break;
                    case 4:
                        force_paint(2, fi);
                        break;
                }
                counter[fi] = 0;
            }

            // score 更新
            if (turn >= TOTAL_TURN / 2) {
                add_score();
            }

            turn += 1;
        }
    }

    // ownerId のみが塗ろうとしているかを判定
    static bool check_counter(unsigned char counter, int owner_id, int idx) {
        return (counter == 1 << idx) || (counter == ((1 << idx) | (1 << owner_id)));
    }

    // score 更新
    void add_score() {
        for (int i = 0; i < 3; ++ i) {
            score[i] += area[i];
        }
    };

    // move 用
    static int func1(int member_id, int pos) {
        int i0 = member_id / 3;
        int i1 = member_id % 3;
        int j0 = pos / 3;
        int j1 = pos % 3;
        return ((j0 + 1) * i1 + j1) % 3 + (i0 + j0) % 2 * 3;
    }

    // field_idx が fi のマスを owner_id が塗る (通常移動)
    void paint(int owner_id, int fi) {
        if (field[fi].owner == -1) {
            // 誰にも塗られていない場合は owner_id で塗る
            area[owner_id] += 1;
            field[fi].owner = owner_id;
            field[fi].val = 2;
        } else if (field[fi].owner == owner_id) {
            // owner_id で塗られている場合は完全に塗られた状態に上書きする
            field[fi].val = 2;
        } else if (field[fi].val == 1) {
            // owner_id 以外で半分塗られた状態の場合は誰にも塗られていない状態にする
            area[field[fi].owner] -= 1;
            field[fi].owner = -1;
            field[fi].val = 0;
        } else {
            // owner_id 以外で完全に塗られた状態の場合は半分塗られた状態にする
            field[fi].val -= 1;
        }
    }

    // field_idx が fi のマスを owner_id が塗る (特殊移動)
    void force_paint(int owner_id, int fi) {
        if (field[fi].owner != owner_id) {
            area[owner_id] += 1;
            if (field[fi].owner != -1) {
                area[field[fi].owner] -= 1;
            }
        }
        field[fi].owner = owner_id;
        field[fi].val = 2;
    }

    // idx のエージェントを v 方向に回転させる
    void rotate_agent(int idx, int v) {
        agents[idx].d += v;
        agents[idx].d %= 4;
    }

    // idx のエージェントを前進させる
    void move_forward(int idx) {
        int i = agents[idx].i;
        int j = agents[idx].j;
        int k = agents[idx].k;
        int d = agents[idx].d;
        int jj = j + Dj[d];
        int kk = k + Dk[d];
        if (jj >= N) {
            agents[idx].i = i/3*3 + (i%3+1)%3; // [1, 2, 0, 4, 5, 3][i]
            agents[idx].j = k;
            agents[idx].k = N - 1;
            agents[idx].d = 3;
        } else if (jj < 0) {
            agents[idx].i = (1-i/3)*3 + (4-i%3)%3; // [4, 3, 5, 1, 0, 2][i]
            agents[idx].j = 0;
            agents[idx].k = N - 1 - k;
            agents[idx].d = 0;
        } else if (kk >= N) {
            agents[idx].i = i/3*3 + (i%3+2)%3; // [2, 0, 1, 5, 3, 4][i]
            agents[idx].j = N - 1;
            agents[idx].k = j;
            agents[idx].d = 2;
        } else if (kk < 0) {
            agents[idx].i = (1-i/3)*3 + (3-i%3)%3; // [3, 5, 4, 0, 2, 1][i]
            agents[idx].j = N - 1 - j;
            agents[idx].k = 0;
            agents[idx].d = 1;
        } else {
            agents[idx].j = jj;
            agents[idx].k = kk;
        }
    }

    static int field_idx(int i, int j, int k) {
        return (i * N + j) * N + k;
    }
};

GameLogic call_move(const string& dir1, const string& dir2) {
    cout << dir1 << " " << dir2 << endl;
    int64_t now;
    GameLogic r;
    cin >> now >> r.turn;
    r.move.resize(6);
    for (auto& x : r.move) {
        cin >> x;
    }
    r.score.resize(3);
    for (auto& x : r.score) {
        cin >> x;
    }
    r.field.resize(6 * N * N);
    r.area.resize(3);
    for (auto& x : r.field) {
        cin >> x.owner >> x.val;
        if (x.owner >= 0) {
            r.area[x.owner] += 1;
        }
    }
    r.agents.resize(6);
    for (auto& x : r.agents) {
        cin >> x.i >> x.j >> x.k >> x.d;
    }
    r.special.resize(6);
    for (auto& x : r.special) {
        cin >> x;
    }
    return r;
}

struct Program {
    static string use_random_special(const string& next_dir) {
        // 50%で直進の必殺技を使用
        if (uniform_int_distribution<>(0, 1)(mt) == 0) {
            return next_dir + "s";
        }
        // 50%でランダムな場所に瞬間移動
        string rand_i = to_string(uniform_int_distribution<>(0, 5)(mt));
        string rand_j = to_string(uniform_int_distribution<>(0, 4)(mt));
        string rand_k = to_string(uniform_int_distribution<>(0, 4)(mt));
        return rand_i + "-" + rand_j + "-" + rand_k;
    }
    void solve() {
        auto next_dir0 = to_string(uniform_int_distribution<>(0, 3)(mt));
        auto next_dir5 = to_string(uniform_int_distribution<>(0, 3)(mt));
        for (;;) {
            // 移動APIを呼ぶ
            auto move = call_move(next_dir0, next_dir5);
            cerr << "turn = " << move.turn << endl;
            cerr << "score = " << move.score[0] << " " << move.score[1] << " " << move.score[2] << endl;
            // 4方向で移動した場合を全部シミュレーションする
            int best_c = -1;
            vector<pair<int, int>> best_d;
            for (int d0 = 0; d0 < 4; ++ d0) {
                for (int d5 = 0; d5 < 4; ++ d5) {
                    auto m = move;
                    m.progress(0, vector<int>{d0, -1, -1, -1, -1, d5});
                    // 自身のエージェントで塗られているマス数をカウントする
                    int c = 0;
                    for (int i = 0; i < 6; ++i) {
                        for (int j = 0; j < N; ++j) {
                            for (int k = 0; k < N; ++k) {
                                if (m.get_cell(i, j, k).owner == 0) {
                                    ++c;
                                }
                            }
                        }
                    }
                    // 最も多くのマスを自身のエージェントで塗れる移動方向のリストを保持する
                    if (c > best_c) {
                        best_c = c;
                        best_d.clear();
                        best_d.emplace_back(d0, d5);
                    } else if (c == best_c) {
                        best_d.emplace_back(d0, d5);
                    }
                }
            }

            // 最も多くのマスを自身のエージェントで塗れる移動方向のリストからランダムで方向を決める
            auto best = best_d[uniform_int_distribution<>(0, (int)best_d.size() - 1)(mt)];
            next_dir0 = to_string(best.first);
            next_dir5 = to_string(best.second);
            // 必殺技の使用回数が残っている場合はランダムな確率で使用する
            if (move.special[0] != 0 and uniform_int_distribution<>(0, 9)(mt) == 0) {
                next_dir0 = use_random_special(next_dir0);
            }
            if (move.special[5] != 0 and uniform_int_distribution<>(0, 9)(mt) == 0) {
                next_dir5 = use_random_special(next_dir5);
            }
        }
    }
};

int main() {
    random_device seed_gen;
    mt = mt19937(seed_gen());

    Program().solve();
}
