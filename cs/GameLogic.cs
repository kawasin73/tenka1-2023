using System.Collections.Generic;
using System.Diagnostics;

public partial class GameLogic
{
    public const int TotalTurn = 294;

    private const int N = 5;
    private static readonly int[] Dj = { +1, 0, -1, 0 };
    private static readonly int[] Dk = { 0, +1, 0, -1 };

    private readonly Cell[] _field;
    private readonly Agent[] _agents;
    private int _turn;
    private readonly int[] _move;
    private readonly int[] _score;
    private readonly int[] _area;
    private readonly int[] _special;

    // moveList に従ってゲームを進行する
    public void Progress(int memberId, IReadOnlyList<int> moveList)
    {
        Debug.Assert(moveList.Count % 6 == 0);
        var counter = new byte[6*N*N];
        var fis = new int[6];
        for (int i = 0; i < moveList.Count; i += 6)
        {
            // エージェントの移動処理
            for (var idx = 0; idx < 6; idx++)
            {
                _move[idx] = moveList[i + Func1(memberId, idx)];
                if (_move[idx] == -1 || _move[idx] >= 4) continue;
                RotateAgent(idx, _move[idx]);
                MoveForward(idx);
                var ii = _agents[idx].I;
                var jj = _agents[idx].J;
                var kk = _agents[idx].K;
                fis[idx] = FieldIdx(ii, jj, kk);
                counter[fis[idx]] |= (byte)(1 << idx);
            }

            // フィールドの更新処理 (通常移動)
            for (var idx = 0; idx < 6; idx++)
            {
                if (_move[idx] == -1 || _move[idx] >= 4) continue;
                var ownerId = idx < 3 ? idx : (5 - idx);
                if (CheckCounter(counter[fis[idx]], ownerId, idx) || _field[fis[idx]].Owner == ownerId)
                {
                    Paint(ownerId, fis[idx]);
                }
            }

            for (var idx = 0; idx < 6; idx++)
            {
                if (_move[idx] == -1 || _move[idx] >= 4) continue;
                counter[fis[idx]] = 0;
            }

            // フィールドの更新処理 (特殊移動)
            var specialFis = new HashSet<int>();
            for (var idx = 0; idx < 6; idx++)
            {
                if (_move[idx] <= 3) continue;
                -- _special[idx];
                var ownerId = idx < 3 ? idx : (5 - idx);
                if (_move[idx] <= 7)
                {
                    // 5 マス前進
                    RotateAgent(idx, _move[idx]);
                    for (var p = 0; p < 5; p++)
                    {
                        MoveForward(idx);
                        var ii = _agents[idx].I;
                        var jj = _agents[idx].J;
                        var kk = _agents[idx].K;
                        var fi = FieldIdx(ii, jj, kk);
                        specialFis.Add(fi);
                        counter[fi] |= (byte)(1 << ownerId);
                    }
                }
                else
                {
                    // 指定したマスに移動
                    var m = _move[idx] - 8;
                    var mi = Func1(ownerId, m / 25);
                    var mj = m / 5 % 5;
                    var mk = m % 5;
                    {
                        var fi = FieldIdx(mi, mj, mk);
                        specialFis.Add(fi);
                        counter[fi] |= (byte)(1 << ownerId);
                    }
                    for (var d = 0; d < 4; d++)
                    {
                        _agents[idx].I = mi;
                        _agents[idx].J = mj;
                        _agents[idx].K = mk;
                        _agents[idx].D = d;
                        MoveForward(idx);
                        var ii = _agents[idx].I;
                        var jj = _agents[idx].J;
                        var kk = _agents[idx].K;
                        var fi = FieldIdx(ii, jj, kk);
                        specialFis.Add(fi);
                        counter[fi] |= (byte)(1 << ownerId);
                    }
                    _agents[idx].I = mi;
                    _agents[idx].J = mj;
                    _agents[idx].K = mk;
                    _agents[idx].D = 0;
                }
            }

            foreach (var fi in specialFis)
            {
                switch (counter[fi])
                {
                    case 1:
                        ForcePaint(0, fi);
                        break;
                    case 2:
                        ForcePaint(1, fi);
                        break;
                    case 4:
                        ForcePaint(2, fi);
                        break;
                }
                counter[fi] = 0;
            }

            // _score 更新
            if (_turn >= TotalTurn / 2)
            {
                AddScore();
            }

            _turn++;
        }
    }

    // ownerId のみが塗ろうとしているかを判定
    private static bool CheckCounter(byte counter, int ownerId, int idx)
    {
        return (counter == 1 << idx) || (counter == ((1 << idx) | (1 << ownerId)));
    }

    // _score 更新
    private void AddScore()
    {
        for (var i = 0; i < 3; i++)
        {
            _score[i] += _area[i];
        }
    }

    // Move 用
    private static int Func1(int memberId, int pos)
    {
        var i0 = memberId / 3;
        var i1 = memberId % 3;
        var j0 = pos / 3;
        var j1 = pos % 3;
        return ((j0 + 1) * i1 + j1) % 3 + (i0 + j0) % 2 * 3;
    }

    // FieldIdx が fi のマスを ownerId が塗る (通常移動)
    private void Paint(int ownerId, int fi)
    {
        if (_field[fi].Owner == -1)
        {
            // 誰にも塗られていない場合は ownerId で塗る
            ++ _area[ownerId];
            _field[fi].Owner = ownerId;
            _field[fi].Val = 2;
        }
        else if (_field[fi].Owner == ownerId)
        {
            // ownerId で塗られている場合は完全に塗られた状態に上書きする
            _field[fi].Val = 2;
        }
        else if (_field[fi].Val == 1)
        {
            // ownerId 以外で半分塗られた状態の場合は誰にも塗られていない状態にする
            -- _area[_field[fi].Owner];
            _field[fi].Owner = -1;
            _field[fi].Val = 0;
        }
        else
        {
            // ownerId 以外で完全に塗られた状態の場合は半分塗られた状態にする
            _field[fi].Val -= 1;
        }
    }

    // FieldIdx が fi のマスを ownerId が塗る (特殊移動)
    private void ForcePaint(int ownerId, int fi)
    {
        if (_field[fi].Owner != ownerId)
        {
            ++ _area[ownerId];
            if (_field[fi].Owner != -1) -- _area[_field[fi].Owner];
        }
        _field[fi].Owner = ownerId;
        _field[fi].Val = 2;
    }

    // idx のエージェントを v 方向に回転させる
    private void RotateAgent(int idx, int v)
    {
        _agents[idx].D += v;
        _agents[idx].D %= 4;
    }

    // idx のエージェントを前進させる
    private void MoveForward(int idx)
    {
        var i = _agents[idx].I;
        var j = _agents[idx].J;
        var k = _agents[idx].K;
        var d = _agents[idx].D;
        var jj = j + Dj[d];
        var kk = k + Dk[d];
        if (jj >= N)
        {
            _agents[idx].I = i / 3 * 3 + (i % 3 + 1) % 3;  // [1, 2, 0, 4, 5, 3][i];
            _agents[idx].J = k;
            _agents[idx].K = N - 1;
            _agents[idx].D = 3;
        }
        else if (jj < 0)
        {
            _agents[idx].I = (1 - i / 3) * 3 + (4 - i % 3) % 3;  // [4, 3, 5, 1, 0, 2][i];
            _agents[idx].J = 0;
            _agents[idx].K = N - 1 - k;
            _agents[idx].D = 0;
        }
        else if (kk >= N)
        {
            _agents[idx].I = i / 3 * 3 + (i % 3 + 2) % 3;  // [2, 0, 1, 5, 3, 4][i];
            _agents[idx].J = N - 1;
            _agents[idx].K = j;
            _agents[idx].D = 2;
        }
        else if (kk < 0)
        {
            _agents[idx].I = (1 - i / 3) * 3 + (3 - i % 3) % 3;  // [3, 5, 4, 0, 2, 1][i];
            _agents[idx].J = N - 1 - j;
            _agents[idx].K = 0;
            _agents[idx].D = 1;
        }
        else
        {
            _agents[idx].J = jj;
            _agents[idx].K = kk;
        }
    }

    // (i, j, k) を _field の添え字にする
    private static int FieldIdx(int i, int j, int k)
    {
        return (i * N + j) * N + k;
    }

    public struct Agent
    {
        public int I;
        public int J;
        public int K;
        public int D;
    }

    public struct Cell
    {
        public int Owner;
        public int Val;
    }
}
