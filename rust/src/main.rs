use std::{thread, time};
use std::collections::HashSet;
use anyhow::Result;
use rand::prelude::*;
use serde::{Serialize, Deserialize};
use thiserror::Error;

const N: i32 = 5;
const N_FIELD: usize = 150; // 6 * N * N
const DJ: [i32; 4] = [1, 0, -1, 0];
const DK: [i32; 4] = [0, 1, 0, -1];
const TOTAL_TURN: i32 = 294;

/// 練習試合開始APIのレスポンス用の構造体
#[derive(Serialize, Deserialize)]
struct StartResponse {
    status: StartStatus,
    game_id: i32,
    start: i64,
}

/// 練習試合の状態
#[derive(Debug, Clone, Copy, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
enum StartStatus {
    Ok,
    Started,

    #[serde(other)]
    Unknown,
}

/// 移動APIのレスポンス用の構造体
#[derive(Serialize, Deserialize)]
struct MoveResponse {
    status: MoveStatus,
    #[serde(default)]
    now: i64,
    #[serde(default)]
    turn: i32,
    #[serde(rename = "move", default)]
    agent_move: [i32; 6],
    #[serde(default)]
    score: [i32; 3],
    #[serde(default)]
    field: [[[[i32; 2]; 5]; 5]; 6],
    #[serde(default)]
    agent: [[i32; 4]; 6],
    #[serde(default)]
    special: [i32; 6],
}

/// 移動APIの結果
#[derive(Debug, Clone, Copy, Serialize, Deserialize)]
#[serde(rename_all = "snake_case")]
pub enum MoveStatus {
    Ok,
    AlreadyMoved,
    GameFinished,

    #[serde(other)]
    Unknown,
}

#[derive(Error, Debug)]
pub enum ApiError {
    #[error("Start Api Error : {0}")]
    StartApiError(String),
    #[error("Api Error")]
    ApiError(),
}

#[derive(Copy, Clone)]
struct Agent {
    i: i32,
    j: i32,
    k: i32,
    d: i32
}

#[derive(Copy, Clone)]
struct Cell {
    owner: i32,
    val: i32
}

struct GameLogic {
    field: [Cell; N_FIELD],
    agents: [Agent; 6],
    turn: i32,
    agent_move: [i32; 6],
    score: [i32; 3],
    area: [i32; 3],
    special: [i32; 6]
}

/// (i, j, k) を field の添え字にする
fn field_idx(i: i32, j: i32, k: i32) -> usize {
    return ((i * N + j) * N + k) as usize;
}

/// move 用
fn func1(member_id: usize, pos: usize) -> usize {
    let i0 = member_id / 3;
    let i1 = member_id % 3;
    let j0 = pos / 3;
    let j1 = pos % 3;
    return ((j0 + 1) * i1 + j1) % 3 + (i0 + j0) % 2 * 3;
}

impl GameLogic {
    fn get_cell(&self, i: i32, j: i32, k: i32) -> &Cell {
        return &self.field[field_idx(i, j, k)];
    }

    /// move_list に従ってゲームを進行する
    fn progress(&mut self, member_id: usize, move_list: Vec<i32>) {
        if move_list.len() % 6 != 0 {
            panic!("invalid moveList length")
        }
        let mut counter = [0u8; N_FIELD];
        let mut fis = [0usize; 6];
        for i in (0..move_list.len()).step_by(6) {
            // エージェントの移動処理
            for idx in 0..6 {
                self.agent_move[idx] = move_list[i + func1(member_id, idx)];
                if self.agent_move[idx] == -1 || self.agent_move[idx] >= 4 {
                    continue;
                }
                self.rotate_agent(idx, self.agent_move[idx]);
                self.move_forward(idx);
                let ii = self.agents[idx].i;
                let jj = self.agents[idx].j;
                let kk = self.agents[idx].k;
                fis[idx] = field_idx(ii, jj, kk);
                counter[fis[idx]] |= 1 << idx;
            }

            // フィールドの更新処理 (通常移動)
            for idx in 0..6 {
                if self.agent_move[idx] == -1 || self.agent_move[idx] >= 4 {
                    continue;
                }
                let owner_id = if idx < 3 { idx } else { 5 - idx };
                if self.check_counter(counter[fis[idx]], owner_id, idx) || self.field[fis[idx]].owner == owner_id as i32 {
                    self.paint(owner_id, fis[idx]);
                }
            }

            for idx in 0..6 {
                if self.agent_move[idx] == -1 || self.agent_move[idx] >= 4 {
                    continue;
                }
                counter[fis[idx]] = 0;
            }

            // フィールドの更新処理 (特殊移動)
            let mut special_fis: HashSet<usize> = HashSet::new();
            for idx in 0..6 {
                if self.agent_move[idx] <= 3 {
                    continue;
                }
                self.special[idx] -= 1;
                let owner_id = if idx < 3 { idx } else { 5 - idx };
                if self.agent_move[idx] <= 7 {
                    // 5 マス前進
                    self.rotate_agent(idx, self.agent_move[idx]);
                    for _ in 0..5 {
                        self.move_forward(idx);
                        let ii = self.agents[idx].i;
                        let jj = self.agents[idx].j;
                        let kk = self.agents[idx].k;
                        let fi = field_idx(ii, jj, kk);
                        special_fis.insert(fi);
                        counter[fi] |= 1 << owner_id;
                    }
                } else {
                    // 指定したマスに移動
                    let m = self.agent_move[idx] - 8;
                    let mi = func1(owner_id, (m / 25) as usize) as i32;
                    let mj = m / 5 % 5;
                    let mk = m % 5;
                    let fi = field_idx(mi, mj, mk);
                    special_fis.insert(fi);
                    counter[fi] |= 1 << owner_id;
                    for d in 0..4 {
                        self.agents[idx].i = mi;
                        self.agents[idx].j = mj;
                        self.agents[idx].k = mk;
                        self.agents[idx].d = d;
                        self.move_forward(idx);
                        let ii = self.agents[idx].i;
                        let jj = self.agents[idx].j;
                        let kk = self.agents[idx].k;
                        let fi = field_idx(ii, jj, kk);
                        special_fis.insert(fi);
                        counter[fi] |= 1 << owner_id;
                    }
                    self.agents[idx].i = mi;
                    self.agents[idx].j = mj;
                    self.agents[idx].k = mk;
                    self.agents[idx].d = 0;
                }
            }

            for fi in special_fis {
                match counter[fi] {
                    1 => self.force_paint(0, fi),
                    2 => self.force_paint(1, fi),
                    4 => self.force_paint(2, fi),
                    _ => {}
                };
                counter[fi] = 0;
            }

            // score 更新
            if self.turn >= TOTAL_TURN / 2 {
                self.add_score();
            }

            self.turn += 1;
        }
    }

    /// owner_id のみが塗ろうとしているかを判定
    fn check_counter(&self, counter: u8, owner_id: usize, idx: usize) -> bool {
        return (counter == 1<<idx) || (counter == ((1 << idx) | (1 << owner_id)));
    }

    /// score 更新
    fn add_score(&mut self) {
        for i in 0..3 {
            self.score[i] += self.area[i];
        }
    }

    /// field_idx が fi のマスを owner_id が塗る (通常移動)
    fn paint(&mut self, owner_id: usize, fi: usize) {
        if self.field[fi].owner == -1 {
            // 誰にも塗られていない場合は owner_id で塗る
            self.area[owner_id] += 1;
            self.field[fi].owner = owner_id as i32;
            self.field[fi].val = 2;
        } else if self.field[fi].owner == owner_id as i32 {
            // owner_id で塗られている場合は完全に塗られた状態に上書きする
            self.field[fi].val = 2;
        } else if self.field[fi].val == 1 {
            // owner_id 以外で半分塗られた状態の場合は誰にも塗られていない状態にする
            self.area[self.field[fi].owner as usize] -= 1;
            self.field[fi].owner = -1;
            self.field[fi].val = 0;
        } else {
            // owner_id 以外で完全に塗られた状態の場合は半分塗られた状態にする
            self.field[fi].val -= 1;
        }
    }

    /// field_idx が fi のマスを owner_id が塗る (特殊移動)
    fn force_paint(&mut self, owner_id: usize, fi: usize) {
        if self.field[fi].owner != owner_id as i32 {
            self.area[owner_id] += 1;
            if self.field[fi].owner != -1 {
                self.area[self.field[fi].owner as usize] -= 1;
            }
        }
        self.field[fi].owner = owner_id as i32;
        self.field[fi].val = 2;
    }

    /// idx のエージェントを v 方向に回転させる
    fn rotate_agent(&mut self, idx: usize, v: i32) {
        self.agents[idx].d += v;
        self.agents[idx].d %= 4;
    }

    /// idx のエージェントを前進させる
    fn move_forward(&mut self, idx: usize) {
        let i = self.agents[idx].i;
        let j = self.agents[idx].j;
        let k = self.agents[idx].k;
        let d = self.agents[idx].d as usize;
        let jj = j + DJ[d];
        let kk = k + DK[d];
        if jj >= N {
            self.agents[idx].i = i / 3 * 3 + (i % 3 + 1) % 3; // [1, 2, 0, 4, 5, 3][i]
            self.agents[idx].j = k;
            self.agents[idx].k = N - 1;
            self.agents[idx].d = 3;
        } else if jj < 0 {
            self.agents[idx].i = (1 - i / 3) * 3 + (4 - i % 3) % 3; // [4, 3, 5, 1, 0, 2][i]
            self.agents[idx].j = 0;
            self.agents[idx].k = N - 1 - k;
            self.agents[idx].d = 0;
        } else if kk >= N {
            self.agents[idx].i = i / 3 * 3 + (i % 3 + 2) % 3; // [2, 0, 1, 5, 3, 4][i]
            self.agents[idx].j = N - 1;
            self.agents[idx].k = j;
            self.agents[idx].d = 2;
        } else if kk < 0 {
            self.agents[idx].i = (1 - i / 3) * 3 + (3 - i % 3) % 3; // [3, 5, 4, 0, 2, 1][i]
            self.agents[idx].j = N - 1 - j;
            self.agents[idx].k = 0;
            self.agents[idx].d = 1;
        } else {
            self.agents[idx].j = jj;
            self.agents[idx].k = kk;
        }
    }
}

fn new_game_logic(move_response: &MoveResponse) -> GameLogic {
    let mut field = [Cell{ owner: 0, val: 0 }; N_FIELD];
    let mut agents = [Agent{ i: 0, j: 0, k: 0, d: 0 }; 6];
    let mut area = [0; 3];
    let mut p_field = 0;
    for i in 0..6 {
        for j in 0..N as usize {
            for k in 0..N as usize {
                field[p_field] = Cell {
                    owner: move_response.field[i][j][k][0],
                    val: move_response.field[i][j][k][1],
                };
                if field[p_field].owner >= 0 {
                    area[field[p_field].owner as usize] += 1;
                }
                p_field += 1;
            }
        }
    }
    for i in 0..6 {
        agents[i] = Agent {
            i: move_response.agent[i][0],
            j: move_response.agent[i][1],
            k: move_response.agent[i][2],
            d: move_response.agent[i][3],
        }
    }
    return GameLogic {
        field,
        agents,
        turn: move_response.turn,
        agent_move: move_response.agent_move.clone(),
        score: move_response.score.clone(),
        area,
        special: move_response.special.clone(),
    }
}

struct Program {
    game_server: String,
    token: String,
    rng: ThreadRng,
    client: reqwest::blocking::Client,
}

impl Program {
    /// ゲームサーバのAPIを叩く
    fn call_api(&self, x: &String) -> Result<reqwest::blocking::Response> {
        let url = format!("{}{}", self.game_server, x);

        // 5xxエラーの際は100ms空けて5回までリトライする
        for _d in 0..5 {
            println!("{url}");

            match self.client.get(&url).send() {
                Ok(response) => {
                    if response.status() == reqwest::StatusCode::OK {
                        return Ok(response);
                    }
                    if response.status().is_server_error() {
                        println!("{}", response.status());
                        let sleep_time = time::Duration::from_millis(100);
                        thread::sleep(sleep_time);
                        continue
                    }
                    Err(ApiError::ApiError())?
                }
                Err(err) => {
                    println!("{}", err);
                    let sleep_time = time::Duration::from_millis(100);
                    thread::sleep(sleep_time);
                    continue
                }
            }
        }

        Err(ApiError::ApiError())?
    }

    /// 指定したmode, delayで練習試合開始APIを呼ぶ
    fn call_start(&self, mode: i32, delay: i32) -> Result<StartResponse> {
        let start_response: StartResponse = self.call_api(&format!("/api/start/{}/{}/{}", self.token, mode, delay))?.json()?;
        Ok(start_response)
    }

    /// dir方向に移動するように移動APIを呼ぶ
    fn call_move(&self, game_id: i32, dir0: &String, dir5: &String) -> Result<MoveResponse> {
        let move_response: MoveResponse = self.call_api(&format!("/api/move/{}/{}/{}/{}", self.token, game_id, dir0, dir5))?.json()?;
        Ok(move_response)
    }

    /// game_idを取得する
    /// 環境変数で指定されていない場合は練習試合のgame_idを返す
    fn get_game_id(&self) -> Result<i32> {
        // 環境変数にGAME_IDが設定されている場合これを優先する
        if let Ok(val) = std::env::var("GAME_ID") {
            return Ok(val.parse()?);
        }

        // start APIを呼び出し練習試合のgame_idを取得する
        let start_response: StartResponse = self.call_start(0, 0)?;
        match start_response.status {
            StartStatus::Ok | StartStatus::Started => Ok(start_response.game_id),
            StartStatus::Unknown => Err(ApiError::StartApiError(serde_json::to_string(&start_response)?).into()),
        }
    }

    // ランダムに必殺技を選択する
    fn use_random_special(&mut self, next_dir: &String) -> String {
        // 50%で直進の必殺技を使用
        if self.rng.gen_bool(0.5) {
            return format!("{}s", next_dir);
        }
        // 50%でランダムな場所に瞬間移動
        let rand_i = self.rng.gen_range(0..4).to_string();
        let rand_j = self.rng.gen_range(0..4).to_string();
        let rand_k = self.rng.gen_range(0..4).to_string();
        return format!("{}-{}-{}", rand_i, rand_j, rand_k);
    }

    /// BOTのメイン処理
    fn solve(&mut self) -> Result<()> {
        self.client = reqwest::blocking::Client::builder().build()?;
        let game_id = self.get_game_id()?;
        let mut next_dir0 = self.rng.gen_range(0..4).to_string();
        let mut next_dir5 = self.rng.gen_range(0..4).to_string();
        loop {
            // 移動APIを呼ぶ
            let move_response = self.call_move(game_id, &next_dir0, &next_dir5)?;
            println!("status = {:?}", move_response.status);
            match move_response.status {
                MoveStatus::AlreadyMoved => continue,
                MoveStatus::Ok => (),
                _ => break,
            }
            println!("turn = {}", move_response.turn);
            println!("score = {:?}", move_response.score);
            // 4方向で移動した場合を全部シミュレーションする
            let mut best_c = -1;
            let mut best_d = [[0; 2]; 16];
            let mut best_d_len = 0;
            for d0 in 0..4 {
                for d5 in 0..4 {
                    let mut m = new_game_logic(&move_response);
                    m.progress(0, vec![d0, -1, -1, -1, -1, d5]);
                    // 自身のエージェントで塗られているマス数をカウントする
                    let mut c = 0;
                    for i in 0..6 {
                        for j in 0..N {
                            for k in 0..N {
                                if m.get_cell(i, j, k).owner == 0 {
                                    c += 1;
                                }
                            }
                        }
                    }
                    // 最も多くのマスを自身のエージェントで塗れる移動方向のリストを保持する
                    if c > best_c {
                        best_c = c;
                        best_d[0] = [d0, d5];
                        best_d_len = 1;
                    } else if c == best_c {
                        best_d[best_d_len] = [d0, d5];
                        best_d_len += 1;
                    }
                }
            }
            // 最も多くのマスを自身のエージェントで塗れる移動方向のリストからランダムで方向を決める
            let best = *best_d[..best_d_len].choose(&mut self.rng).unwrap_or(&[0, 0]);
            next_dir0 = best[0].to_string();
            next_dir5 = best[1].to_string();
            // 必殺技の使用回数が残っている場合はランダムな確率で使用する
            if move_response.special[0] != 0 && self.rng.gen_bool(0.1) {
                next_dir0 = self.use_random_special(&next_dir0);
            }
            if move_response.special[5] != 0 && self.rng.gen_bool(0.1) {
                next_dir5 = self.use_random_special(&next_dir5);
            }
        }

        Ok(())
    }
}

fn main() {
    let mut program = Program {
        game_server: std::env::var("GAME_SERVER").unwrap_or_else(|_| "https://gbc2023.tenka1.klab.jp".to_string()),
        token: std::env::var("TOKEN").unwrap_or_else(|_| "YOUR_TOKEN".to_string()),
        rng: thread_rng(),
        client: Default::default(),
    };
    match program.solve() {
        Ok(_) => println!("finish"),
        Err(msg) => panic!("failure: {}", msg),
    }
}
