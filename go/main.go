package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strconv"
	"time"
)

var GameServer = "https://gbc2023.tenka1.klab.jp"
var TOKEN = "YOUR_TOKEN"

const N = 5
const TOTAL_TURN = 294

var Dj = []int{+1, 0, -1, 0}
var Dk = []int{0, +1, 0, -1}

// 初期化処理
func init() {
	rand.Seed(time.Now().Unix())

	if os.Getenv("GAME_SERVER") != "" {
		GameServer = os.Getenv("GAME_SERVER")
	}
	if os.Getenv("TOKEN") != "" {
		TOKEN = os.Getenv("TOKEN")
	}
}

// ゲームサーバのAPIを叩く
func callAPI(x string) ([]byte, error) {
	url := GameServer + x
	// err != nilの場合 または 5xxエラーの際は100ms空けて5回までリトライする
	for i := 0; i < 5; i++ {
		fmt.Println(url)
		resp, err := http.Get(url)
		if err != nil {
			log.Printf("%v", err.Error())
			time.Sleep(time.Millisecond * 100)
			continue
		}
		//goland:noinspection GoUnhandledErrorResult
		defer resp.Body.Close()
		body, err := ioutil.ReadAll(resp.Body)
		if resp.StatusCode == 200 {
			return body, nil
		}
		if 500 <= resp.StatusCode && resp.StatusCode < 600 {
			fmt.Println(resp.Status)
			time.Sleep(time.Millisecond * 100)
			continue
		}

		return nil, fmt.Errorf("Api Error status_code:%d", resp.StatusCode)
	}
	return nil, fmt.Errorf("Api Error")
}

// 移動APIのレスポンス用の構造体
type StartResponse struct {
	Status string `json:"status"`
	Start  int64  `json:"start"`
	GameId int64  `json:"game_id"`
}

// 指定したmode, delayで練習試合開始APIを呼ぶ
func callStart(mode, delay int) *StartResponse {
	res, err := callAPI(fmt.Sprintf("/api/start/%s/%d/%d", TOKEN, mode, delay))
	if err != nil {
		log.Fatal(err)
	}
	var move StartResponse
	err = json.Unmarshal(res, &move)
	if err != nil {
		log.Fatal(err)
	}
	return &move
}

// 移動APIのレスポンス用の構造体
type MoveResponse struct {
	Status  string      `json:"status"`
	Now     int64       `json:"now"`
	Turn    int         `json:"turn"`
	Move    []int       `json:"move"`
	Score   []int       `json:"score"`
	Field   [][][][]int `json:"field"`
	Agent   [][]int     `json:"agent"`
	Special []int       `json:"special"`
}

// dir方向に移動するように移動APIを呼ぶ
func callMove(gameId int64, dir0, dir5 string) *MoveResponse {
	res, err := callAPI(fmt.Sprintf("/api/move/%s/%d/%s/%s", TOKEN, gameId, dir0, dir5))
	if err != nil {
		log.Fatal(err)
	}
	var move MoveResponse
	err = json.Unmarshal(res, &move)
	if err != nil {
		log.Fatal(err)
	}
	return &move
}

// game_idを取得する
// 環境変数で指定されていない場合は練習試合のgame_idを返す
func getGameId() int64 {
	// 環境変数にGAME_IDが設定されている場合これを優先する
	if os.Getenv("GAME_ID") != "" {
		r, err := strconv.ParseInt(os.Getenv("GAME_ID"), 10, 64)
		if err != nil {
			log.Fatal(err)
		}
		return r
	}

	// start APIを呼び出し練習試合のgame_idを取得する
	start := callStart(0, 0)
	if start.Status == "ok" || start.Status == "started" {
		return start.GameId
	}

	log.Fatal(fmt.Sprintf("Start Api Error : %s", start.Status))
	return -1
}

// (i, j, k) を Field の添え字にする
func FieldIdx(i, j, k int) int {
	return (i*N+j)*N + k
}

// Move 用
func Func1(memberId, pos int) int {
	i0 := memberId / 3
	i1 := memberId % 3
	j0 := pos / 3
	j1 := pos % 3
	return ((j0+1)*i1+j1)%3 + (i0+j0)%2*3
}

type Agent struct {
	I int
	J int
	K int
	D int
}

type Cell struct {
	Owner int
	Val   int
}

type GameLogic struct {
	Field   []*Cell
	Agents  []*Agent
	Turn    int
	Move    []int
	Score   []int
	Area    []int
	Special []int
}

func (g *GameLogic) GetCell(i, j, k int) *Cell {
	return g.Field[FieldIdx(i, j, k)]
}

// moveList に従ってゲームを進行する
func (g *GameLogic) Progress(memberId int, moveList []int) {
	if len(moveList)%6 != 0 {
		log.Fatal("invalid moveList length")
	}
	counter := make([]byte, 6*N*N)
	fis := make([]int, 6)
	for i := 0; i < len(moveList); i += 6 {
		// エージェントの移動処理
		for idx := 0; idx < 6; idx++ {
			g.Move[idx] = moveList[i+Func1(memberId, idx)]
			if g.Move[idx] == -1 || g.Move[idx] >= 4 {
				continue
			}
			g.RotateAgent(idx, g.Move[idx])
			g.MoveForward(idx)
			ii := g.Agents[idx].I
			jj := g.Agents[idx].J
			kk := g.Agents[idx].K
			fis[idx] = FieldIdx(ii, jj, kk)
			counter[fis[idx]] |= 1 << idx
		}

		// フィールドの更新処理 (通常移動)
		for idx := 0; idx < 6; idx++ {
			if g.Move[idx] == -1 || g.Move[idx] >= 4 {
				continue
			}
			var ownerId int
			if idx < 3 {
				ownerId = idx
			} else {
				ownerId = 5 - idx
			}
			if g.CheckCounter(counter[fis[idx]], ownerId, idx) || g.Field[fis[idx]].Owner == ownerId {
				g.Paint(ownerId, fis[idx])
			}
		}

		for idx := 0; idx < 6; idx++ {
			if g.Move[idx] == -1 || g.Move[idx] >= 4 {
				continue
			}
			counter[fis[idx]] = 0
		}

		// フィールドの更新処理 (特殊移動)
		specialFis := make(map[int]bool)
		for idx := 0; idx < 6; idx++ {
			if g.Move[idx] <= 3 {
				continue
			}
			g.Special[idx] -= 1
			var ownerId int
			if idx < 3 {
				ownerId = idx
			} else {
				ownerId = 5 - idx
			}
			if g.Move[idx] <= 7 {
				// 5 マス前進
				g.RotateAgent(idx, g.Move[idx])
				for p := 0; p < 5; p++ {
					g.MoveForward(idx)
					ii := g.Agents[idx].I
					jj := g.Agents[idx].J
					kk := g.Agents[idx].K
					fi := FieldIdx(ii, jj, kk)
					specialFis[fi] = true
					counter[fi] |= 1 << ownerId
				}
			} else {
				// 指定したマスに移動
				m := g.Move[idx] - 8
				mi := Func1(ownerId, m/25)
				mj := m / 5 % 5
				mk := m % 5
				{
					fi := FieldIdx(mi, mj, mk)
					specialFis[fi] = true
					counter[fi] |= 1 << ownerId
				}
				for d := 0; d < 4; d++ {
					g.Agents[idx].I = mi
					g.Agents[idx].J = mj
					g.Agents[idx].K = mk
					g.Agents[idx].D = d
					g.MoveForward(idx)
					ii := g.Agents[idx].I
					jj := g.Agents[idx].J
					kk := g.Agents[idx].K
					fi := FieldIdx(ii, jj, kk)
					specialFis[fi] = true
					counter[fi] |= 1 << ownerId
				}
				g.Agents[idx].I = mi
				g.Agents[idx].J = mj
				g.Agents[idx].K = mk
				g.Agents[idx].D = 0
			}
		}

		for fi := range specialFis {
			switch counter[fi] {
			case 1:
				g.ForcePaint(0, fi)
				break
			case 2:
				g.ForcePaint(1, fi)
				break
			case 4:
				g.ForcePaint(2, fi)
				break
			}
			counter[fi] = 0
		}

		// Score 更新
		if g.Turn >= TOTAL_TURN/2 {
			g.AddScore()
		}

		g.Turn += 1
	}
}

// ownerId のみが塗ろうとしているかを判定
func (g *GameLogic) CheckCounter(counter byte, ownerId, idx int) bool {
	return (counter == 1<<idx) || (counter == ((1 << idx) | (1 << ownerId)))
}

// Score 更新
func (g *GameLogic) AddScore() {
	for i := 0; i < 3; i++ {
		g.Score[i] += g.Area[i]
	}
}

// FieldIdx が fi のマスを ownerId が塗る (通常移動)
func (g *GameLogic) Paint(ownerId, fi int) {
	if g.Field[fi].Owner == -1 {
		// 誰にも塗られていない場合は ownerId で塗る
		g.Area[ownerId] += 1
		g.Field[fi].Owner = ownerId
		g.Field[fi].Val = 2
	} else if g.Field[fi].Owner == ownerId {
		// ownerId で塗られている場合は完全に塗られた状態に上書きする
		g.Field[fi].Val = 2
	} else if g.Field[fi].Val == 1 {
		// ownerId 以外で半分塗られた状態の場合は誰にも塗られていない状態にする
		g.Area[g.Field[fi].Owner] -= 1
		g.Field[fi].Owner = -1
		g.Field[fi].Val = 0
	} else {
		// ownerId 以外で完全に塗られた状態の場合は半分塗られた状態にする
		g.Field[fi].Val -= 1
	}
}

// FieldIdx が fi のマスを ownerId が塗る (特殊移動)
func (g *GameLogic) ForcePaint(ownerId, fi int) {
	if g.Field[fi].Owner != ownerId {
		g.Area[ownerId] += 1
		if g.Field[fi].Owner != -1 {
			g.Area[g.Field[fi].Owner] -= 1
		}
	}
	g.Field[fi].Owner = ownerId
	g.Field[fi].Val = 2
}

// idx のエージェントを v 方向に回転させる
func (g *GameLogic) RotateAgent(idx, v int) {
	g.Agents[idx].D += v
	g.Agents[idx].D %= 4
}

// idx のエージェントを前進させる
func (g *GameLogic) MoveForward(idx int) {
	i := g.Agents[idx].I
	j := g.Agents[idx].J
	k := g.Agents[idx].K
	d := g.Agents[idx].D
	jj := j + Dj[d]
	kk := k + Dk[d]
	if jj >= N {
		g.Agents[idx].I = i/3*3 + (i%3+1)%3 // [1, 2, 0, 4, 5, 3][i];
		g.Agents[idx].J = k
		g.Agents[idx].K = N - 1
		g.Agents[idx].D = 3
	} else if jj < 0 {
		g.Agents[idx].I = (1-i/3)*3 + (4-i%3)%3 // [4, 3, 5, 1, 0, 2][i];
		g.Agents[idx].J = 0
		g.Agents[idx].K = N - 1 - k
		g.Agents[idx].D = 0
	} else if kk >= N {
		g.Agents[idx].I = i/3*3 + (i%3+2)%3 // [2, 0, 1, 5, 3, 4][i];
		g.Agents[idx].J = N - 1
		g.Agents[idx].K = j
		g.Agents[idx].D = 2
	} else if kk < 0 {
		g.Agents[idx].I = (1-i/3)*3 + (3-i%3)%3 // [3, 5, 4, 0, 2, 1][i];
		g.Agents[idx].J = N - 1 - j
		g.Agents[idx].K = 0
		g.Agents[idx].D = 1
	} else {
		g.Agents[idx].J = jj
		g.Agents[idx].K = kk
	}
}

func NewGameLogic(move *MoveResponse) *GameLogic {
	field := make([]*Cell, 6*N*N)
	agents := make([]*Agent, 6)
	area := make([]int, 3)
	for i := 0; i < 6; i++ {
		for j := 0; j < N; j++ {
			for k := 0; k < N; k++ {
				owner := move.Field[i][j][k][0]
				field[FieldIdx(i, j, k)] = &Cell{
					Owner: owner,
					Val:   move.Field[i][j][k][1],
				}
				if owner >= 0 {
					area[owner] += 1
				}
			}
		}
	}
	for i := 0; i < 6; i++ {
		agents[i] = &Agent{
			I: move.Agent[i][0],
			J: move.Agent[i][1],
			K: move.Agent[i][2],
			D: move.Agent[i][3],
		}
	}
	return &GameLogic{
		Field:   field,
		Agents:  agents,
		Turn:    move.Turn,
		Move:    append([]int{}, move.Move...),
		Score:   append([]int{}, move.Score...),
		Area:    area,
		Special: append([]int{}, move.Special...),
	}
}

type Program struct {
}

func NewProgram() *Program {
	return &Program{}
}

func (bot *Program) useRandomSpecial(nextDir string) string {
	// 50%で直進の必殺技を使用
	if rand.Intn(2) == 0 {
		return nextDir + "s"
	}
	// 50%でランダムな場所に瞬間移動
	i := rand.Intn(6)
	j := rand.Intn(5)
	k := rand.Intn(5)
	return fmt.Sprintf("%d-%d-%d", i, j, k)
}

// BOTのメイン処理
func (bot *Program) solve() {
	gameId := getGameId()
	nextDir0 := strconv.Itoa(rand.Intn(4))
	nextDir5 := strconv.Itoa(rand.Intn(4))
	for {
		// 移動APIを呼ぶ
		move := callMove(gameId, nextDir0, nextDir5)
		log.Printf("status = %s\n", move.Status)
		if move.Status == "already_moved" {
			continue
		} else if move.Status != "ok" {
			break
		}
		log.Printf("turn = %d", move.Turn)
		log.Printf("score = %d %d %d", move.Score[0], move.Score[1], move.Score[2])
		// 4方向で移動した場合を全部シミュレーションする
		type dirPair struct {
			dir0, dir5 int
		}
		bestC := -1
		var bestD []dirPair
		for d0 := 0; d0 < 4; d0++ {
			for d5 := 0; d5 < 4; d5++ {
				m := NewGameLogic(move)
				m.Progress(0, []int{d0, -1, -1, -1, -1, d5})
				// 自身のエージェントで塗られているマス数をカウントする
				c := 0
				for i := 0; i < 6; i++ {
					for j := 0; j < N; j++ {
						for k := 0; k < N; k++ {
							if m.GetCell(i, j, k).Owner == 0 {
								c++
							}
						}
					}
				}
				// 最も多くのマスを自身のエージェントで塗れる移動方向のリストを保持する
				if c > bestC {
					bestC = c
					bestD = nil
					bestD = append(bestD, dirPair{d0, d5})
				} else if c == bestC {
					bestD = append(bestD, dirPair{d0, d5})
				}
			}
		}
		// 最も多くのマスを自身のエージェントで塗れる移動方向のリストからランダムで方向を決める
		best := bestD[rand.Intn(len(bestD))]
		nextDir0 = strconv.Itoa(best.dir0)
		nextDir5 = strconv.Itoa(best.dir5)
		// 必殺技を使っていない場合はランダムな確率で使用する
		if move.Special[0] != 0 && rand.Intn(100) < 10 {
			nextDir0 = bot.useRandomSpecial(nextDir0)
		}
		if move.Special[5] != 0 && rand.Intn(100) < 10 {
			nextDir5 = bot.useRandomSpecial(nextDir5)
		}
	}
}

func main() {
	NewProgram().solve()
}
