package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"net/http"
	"os"
	"slices"
	"sort"
	"strconv"
	"time"
)

var GameServer = "https://gbc2023.tenka1.klab.jp"
var TOKEN = "YOUR_TOKEN"

const N = 5
const TOTAL_TURN = 294
const N_AGENTS = 4

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

func MoveRotation(pos []int, rotation int) []int {
	nextPos := make([]int, 4)
	for i := 0; i < 4; i++ {
		nextPos[i] = pos[i]
	}
	nextPos[3] += rotation
	nextPos[3] %= 4
	MoveForward(nextPos)
	return nextPos
}

func MoveForward(pos []int) {
	i := pos[0]
	j := pos[1]
	k := pos[2]
	d := pos[3]
	var jj = j + Dj[d]
	var kk = k + Dk[d]
	if jj >= N {
		pos[0] = i/3*3 + (i%3+1)%3 // [1, 2, 0, 4, 5, 3][i]
		pos[1] = k
		pos[2] = N - 1
		pos[3] = 3
	} else if jj < 0 {
		pos[0] = (1-i/3)*3 + (4-i%3)%3 // [4, 3, 5, 1, 0, 2][i]
		pos[1] = 0
		pos[2] = N - 1 - k
		pos[3] = 0
	} else if kk >= N {
		pos[0] = i/3*3 + (i%3+2)%3 // [2, 0, 1, 5, 3, 4][i]
		pos[1] = N - 1
		pos[2] = j
		pos[3] = 2
	} else if kk < 0 {
		pos[0] = (1-i/3)*3 + (3-i%3)%3 // [3, 5, 4, 0, 2, 1][i]
		pos[1] = N - 1 - j
		pos[2] = 0
		pos[3] = 1
	} else {
		pos[1] = jj
		pos[2] = kk
	}
}

// エージェントが同じマスにいるかを判定する
func IsSamePos(a []int, b []int) bool {
	return a[0] == b[0] && a[1] == b[1] && a[2] == b[2]
}

// エージェントが同じマスにいるかを判定する
func IsSameState(a []int, b []int) bool {
	return a[0] == b[0] && a[1] == b[1]
}

func createMap() [][][]int {
	m := make([][][]int, 6)
	for i := 0; i < 6; i++ {
		m[i] = make([][]int, N)
		for j := 0; j < N; j++ {
			m[i][j] = make([]int, N)
		}
	}
	return m
}

var agentMap = []int{0, 1, 2, 2, 1, 0}

func Agent2Player(agent int) int {
	return agentMap[agent]
}

func (m *MoveResponse) EnemiesInLength(pos []int, l int) []int {
	result := make([]int, 0, N_AGENTS)
	visited := createMap()
	visited[pos[0]][pos[1]][pos[2]] = -1

	if l == 0 {
		for agent, agentPos := range m.Agent[1:5] {
			if IsSamePos(pos, agentPos) {
				result = append(result, agent+1)
			}
		}
		return result
	}

	var current [][]int
	for d := 0; d < 4; d++ {
		nextPos := MoveRotation(pos, d)
		visited[nextPos[0]][nextPos[1]][nextPos[2]] = 1

		if l == 1 {
			for agent, agentPos := range m.Agent[1:5] {
				if IsSamePos(nextPos, agentPos) {
					result = append(result, agent+1)
				}
			}
		}
		current = append(current, nextPos)
	}
	if l == 1 {
		return result
	}
	var next [][]int

	for length := 2; length <= l; length++ {
		for _, pos := range current {
			for d := 0; d < 4; d++ {
				nextPos := MoveRotation(pos, d)
				if visited[nextPos[0]][nextPos[1]][nextPos[2]] == 0 {
					visited[nextPos[0]][nextPos[1]][nextPos[2]] = length
					next = append(next, nextPos)

					if length == l {
						for agent, agentPos := range m.Agent[1:5] {
							if IsSamePos(nextPos, agentPos) {
								result = append(result, agent+1)
							}
						}
					}
				}
			}
		}
		current = next
		next = nil
	}
	return result
}

type Length2Prediction struct {
	numEmpty  int
	numEmpty0 int
	numEmpty1 int
	numEmpty2 int
}

func CreateLength2Prediction(move *MoveResponse, agent int, pos []int) Length2Prediction {
	var length2List [][]int
	for d := 0; d < 4; d++ {
		length2Pos := MoveRotation(pos, d)
		if !IsSamePos(length2Pos, move.Agent[agent]) {
			length2List = append(length2List, length2Pos)
		}
	}
	var prediction Length2Prediction
	for _, pos := range length2List {
		if move.Field[pos[0]][pos[1]][pos[2]][0] == -1 {
			if len(move.EnemiesInLength(pos, 1)) > 0 {
				prediction.numEmpty1++
			} else if len(move.EnemiesInLength(pos, 0)) > 0 {
				prediction.numEmpty0++
			} else if len(move.EnemiesInLength(pos, 2)) > 0 {
				prediction.numEmpty2++
			} else {
				prediction.numEmpty++
			}
		}
	}
	return prediction
}

func (p *Length2Prediction) TotalEmpty() int {
	return p.numEmpty + p.numEmpty0 + p.numEmpty1 + p.numEmpty2
}

func (p *Length2Prediction) LessThan(p2 *Length2Prediction) bool {
	if p.TotalEmpty() != p2.TotalEmpty() {
		return p.TotalEmpty() < p2.TotalEmpty()
	}
	if p.numEmpty != p2.numEmpty {
		return p.numEmpty < p2.numEmpty
	}
	if p.numEmpty2 != p2.numEmpty2 {
		return p.numEmpty2 < p2.numEmpty2
	}
	if p.numEmpty0 != p2.numEmpty0 {
		return p.numEmpty0 < p2.numEmpty0
	}
	return p.numEmpty1 < p2.numEmpty1
}

type ShortTermPrediction struct {
	priority     ShortTermPredictionValue
	damageTarget bool
}

func (p *ShortTermPrediction) IsSame(p2 *ShortTermPrediction) bool {
	return p.priority == p2.priority && p.damageTarget == p2.damageTarget
}

func (p *ShortTermPrediction) LessThan(p2 *ShortTermPrediction) bool {
	if p.priority != p2.priority {
		return p.priority < p2.priority
	}
	return p.damageTarget && !p2.damageTarget
}

func NewShortTermPrediction(move *MoveResponse, pos []int, target int) ShortTermPrediction {
	enemies0 := move.EnemiesInLength(pos, 0)
	for i, e := range enemies0 {
		enemies0[i] = Agent2Player(e)
	}
	enemies1 := move.EnemiesInLength(pos, 1)
	for i, e := range enemies1 {
		enemies1[i] = Agent2Player(e)
	}
	enemies2 := move.EnemiesInLength(pos, 2)
	for i, e := range enemies2 {
		log.Println("enemy2", e, Agent2Player(e))
		enemies2[i] = Agent2Player(e)
	}

	state := move.Field[pos[0]][pos[1]][pos[2]]

	priority := Others
	damageTarget := false
	if state[0] == -1 {
		priority = Empty
		if len(enemies1) > 0 {
			priority = EmptyMayConflict
			damageTarget = slices.Contains(enemies1, target)
		} else if len(enemies0) > 0 {
			priority = EmptySteal
			damageTarget = slices.Contains(enemies0, target)
		} else if len(enemies2) > 0 {
			log.Println("empty may stolen", pos, enemies2)
			log.Println("empty may stolen", move)
			priority = EmptyMayStolen
		}
	} else if state[0] == 0 {
		if state[1] == 1 {
			priority = SelfHalf
			if len(enemies1) > 0 {
				priority = SelfHalfMayConflict
				damageTarget = slices.Contains(enemies1, target)
			} else if len(enemies0) > 0 || len(enemies2) > 0 {
				priority = SelfHalfRecover
			}
		} else if state[1] == 2 {
			priority = SelfFull
			if len(enemies1) > 0 {
				priority = SelfFullMayConflict
			}
		} else {
			log.Println("error: self unexpected state", state[1])
		}
	} else {
		if state[1] == 1 {
			priority = EnemyHalf
			if len(enemies0) > 0 {
				priority = Dont
			} else if slices.Contains(enemies1, state[0]) {
				priority = EnemyHalfMayConflictEnemy
				damageTarget = slices.Contains(enemies1, target)
			} else if len(enemies1) > 0 {
				priority = EnemyHalfMayConflict
				damageTarget = slices.Contains(enemies1, target)
			} else if len(enemies2) > 0 {
				priority = EnemyHalfMayStolen
				damageTarget = slices.Contains(enemies2, target)
			}
		} else if state[1] == 2 {
			priority = EnemyFull
			if slices.Contains(enemies0, state[0]) {
				priority = Dont
			} else if slices.Contains(enemies1, state[0]) {
				priority = EnemyFullMayRecovered
				damageTarget = slices.Contains(enemies1, target)
			} else if len(enemies1) > 0 {
				priority = EnemyFullMayConflict
				damageTarget = slices.Contains(enemies1, target)
			} else if len(enemies2) > 0 && !slices.Contains(enemies2, state[0]) {
				priority = EnemyFullCanSteal
				damageTarget = slices.Contains(enemies2, target)
			}
		} else {
			log.Println("error: enemy unexpected state", state[1])
		}
	}

	return ShortTermPrediction{
		priority:     priority,
		damageTarget: damageTarget,
	}
}

type ShortTermPredictionValue int

const (
	EmptySteal ShortTermPredictionValue = iota
	EmptyMayStolen
	Empty
	EmptyMayConflict
	SelfHalfMayConflict
	EnemyHalf
	EnemyHalfMayConflict
	SelfHalfRecover
	EnemyFullCanSteal
	EnemyFull
	SelfHalf
	EnemyFullMayConflict
	EnemyHalfMayStolen // TODO
	EnemyHalfMayConflictEnemy
	SelfFullMayConflict
	EnemyFullMayRecovered
	SelfFull
	Others
	Dont
)

func CalcPotential(move *MoveResponse, target int, lengthMap [][][]int) int {
	potential := 0
	for i := 0; i < 6; i++ {
		for j := 0; j < N; j++ {
			for k := 0; k < N; k++ {
				length := lengthMap[i][j][k]
				if length > 0 {
					state := move.Field[i][j][k]
					if state[1] == 0 {
						potential += (11 - length) * 10
					} else if state[0] != 0 && state[1] == 1 {
						potential += (11 - length) * 8
						if state[0] == target {
							potential += (11 - length) * 4
						}
					} else if state[0] != 0 && state[1] == 2 {
						potential += (11 - length) * 5
						if state[0] == target {
							potential += (11 - length) * 4
						}
					}
				}
			}
		}
	}
	return potential
}

type Prediction struct {
	pos                 []int
	rotation            int
	shortTermPrediction ShortTermPrediction
	length2Prediction   Length2Prediction
	potential           int
}

func CreatePrediction(move *MoveResponse, agent int, rotation int, target int, directionMap [][][][]int) Prediction {
	pos := MoveRotation(move.Agent[agent], rotation)

	return Prediction{
		pos:                 pos,
		rotation:            rotation,
		shortTermPrediction: NewShortTermPrediction(move, pos, target),
		length2Prediction:   CreateLength2Prediction(move, agent, pos),
		potential:           CalcPotential(move, target, directionMap[rotation]),
	}
}

// left -> straight -> right -> back
var rotationOrderEmpty = []int{1, 0, 2, 3}

func (p *Prediction) LessThan(p2 *Prediction) bool {
	if p.shortTermPrediction.priority > SelfHalfMayConflict && p2.shortTermPrediction.priority > SelfHalfMayConflict {
		if p.length2Prediction.TotalEmpty() > 0 || p2.length2Prediction.TotalEmpty() > 0 {
			return !p.length2Prediction.LessThan(&p2.length2Prediction)
		}
	}
	if !p.shortTermPrediction.IsSame(&p2.shortTermPrediction) {
		return p.shortTermPrediction.LessThan(&p2.shortTermPrediction)
	}
	if p.shortTermPrediction.priority > EmptyMayConflict && p.length2Prediction.TotalEmpty() > 0 || p2.length2Prediction.TotalEmpty() > 0 {
		return !p.length2Prediction.LessThan(&p2.length2Prediction)
	}
	if p.shortTermPrediction.priority <= EmptyMayConflict {
		return rotationOrderEmpty[p.rotation] < rotationOrderEmpty[p2.rotation]
	}
	return p.rotation < p2.rotation
}

type Rank struct {
	player    int
	point     int
	cellPoint int
}

func (m *MoveResponse) EstimateRanking() []Rank {
	ranking := make([]Rank, 3)
	for i := 0; i < 3; i++ {
		ranking[i].player = i
		ranking[i].point = m.Score[i]
	}
	leftTurns := 294 - m.Turn
	for i := 0; i < 6; i++ {
		for j := 0; j < N; j++ {
			for k := 0; k < N; k++ {
				if m.Field[i][j][k][0] != -1 {
					ranking[m.Field[i][j][k][0]].cellPoint++
				}
			}
		}
	}
	for i := 0; i < 3; i++ {
		ranking[i].point += ranking[i].cellPoint * leftTurns
	}
	sort.Slice(ranking, func(i, j int) bool {
		if ranking[i].point != ranking[j].point {
			return ranking[i].point > ranking[j].point
		}
		return ranking[i].cellPoint < ranking[j].cellPoint
	})
	return ranking
}

type SpecialPrediction struct {
	isStraight  bool
	pos         []int
	direction   int
	nTargetFull int
	nTargetHalf int
	nEnemyFull  int
	nEmpty      int
	nSelf       int
}

func NewSpecialPredictionStraight(move *MoveResponse, pos []int, direction int, target int) SpecialPrediction {
	p := SpecialPrediction{
		isStraight: true,
		pos:        pos,
		direction:  direction,
	}
	pos = MoveRotation(pos, direction)

	for i := 0; i < 5; i++ {
		state := move.Field[pos[0]][pos[1]][pos[2]]
		if state[0] == -1 {
			p.nEmpty++
		} else if state[0] == 0 {
			p.nSelf++
		} else if state[0] == target {
			if state[1] == 1 {
				p.nTargetHalf++
			} else if state[1] == 2 {
				p.nTargetFull++
			}
		} else {
			if state[1] == 2 {
				p.nEnemyFull++
			}
		}
		MoveForward(pos)
	}
	return p
}

func (p *SpecialPrediction) ApiCall() string {
	if p.isStraight {
		return strconv.Itoa(p.direction) + "s"
	} else {
		return ""
	}
}

func (p *SpecialPrediction) ShouldSkip() bool {
	return p.nSelf > 0 || p.nEmpty > 1
}

func (p *SpecialPrediction) Point() int {
	return p.nTargetFull*4 + p.nTargetHalf*2 + p.nEnemyFull - p.nEmpty
}

func (move *MoveResponse) CreateDirectionMap(agent int) [][][][]int {
	return CreateDirectionLengthMap(move.Agent[agent])
}

func CreateDirectionLengthMap(pos []int) [][][][]int {
	maps := make([][][][]int, 4)
	for d := 0; d < 4; d++ {
		maps[d] = createMap()
		maps[d][pos[0]][pos[1]][pos[2]] = -1
	}

	current := make([][][]int, 4)
	for d := 0; d < 4; d++ {
		nextPos := MoveRotation(pos, d)
		maps[d][nextPos[0]][nextPos[1]][nextPos[2]] = 1
		current[d] = append(current[d], nextPos)
	}
	next := make([][][]int, 4)

	for length := 2; ; length++ {
		for dd := 0; dd < 4; dd++ {
			for _, pos := range current[dd] {
				for i := 0; i < 4; i++ {
					nextPos := MoveRotation(pos, i)
					found := false
					for _, m := range maps {
						if m[nextPos[0]][nextPos[1]][nextPos[2]] != 0 {
							found = true
							break
						}
					}
					if !found {
						next[dd] = append(next[dd], nextPos)
					}
				}
			}
		}
		for dd := 0; dd < 4; dd++ {
			for _, v := range next[dd] {
				maps[dd][v[0]][v[1]][v[2]] = length
			}
		}
		if len(next[0]) == 0 && len(next[1]) == 0 && len(next[2]) == 0 && len(next[3]) == 0 {
			break
		}
		current = next
		next = make([][][]int, 4)
	}
	return maps
}

func printMap(m [][][]int) {
	for i := 0; i < 6; i++ {
		for j := 0; j < N; j++ {
			fmt.Println(m[i][j])
		}
		fmt.Println()
	}
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

	var fieldLog [][][][][]int
	var agentLog [][][]int

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

		start := time.Now()

		// // 4方向で移動した場合を全部シミュレーションする
		// type dirPair struct {
		// 	dir0, dir5 int
		// }
		// bestC := -1
		// var bestD []dirPair
		// for d0 := 0; d0 < 4; d0++ {
		// 	for d5 := 0; d5 < 4; d5++ {
		// 		m := NewGameLogic(move)
		// 		m.Progress(0, []int{d0, -1, -1, -1, -1, d5})
		// 		// 自身のエージェントで塗られているマス数をカウントする
		// 		c := 0
		// 		for i := 0; i < 6; i++ {
		// 			for j := 0; j < N; j++ {
		// 				for k := 0; k < N; k++ {
		// 					if m.GetCell(i, j, k).Owner == 0 {
		// 						c++
		// 					}
		// 				}
		// 			}
		// 		}
		// 		// 最も多くのマスを自身のエージェントで塗れる移動方向のリストを保持する
		// 		if c > bestC {
		// 			bestC = c
		// 			bestD = nil
		// 			bestD = append(bestD, dirPair{d0, d5})
		// 		} else if c == bestC {
		// 			bestD = append(bestD, dirPair{d0, d5})
		// 		}
		// 	}
		// }
		// // 最も多くのマスを自身のエージェントで塗れる移動方向のリストからランダムで方向を決める
		// best := bestD[rand.Intn(len(bestD))]
		// nextDir0 = strconv.Itoa(best.dir0)
		// nextDir5 = strconv.Itoa(best.dir5)
		// // 必殺技を使っていない場合はランダムな確率で使用する
		// if move.Special[0] != 0 && rand.Intn(100) < 10 {
		// 	nextDir0 = bot.useRandomSpecial(nextDir0)
		// }
		// if move.Special[5] != 0 && rand.Intn(100) < 10 {
		// 	nextDir5 = bot.useRandomSpecial(nextDir5)
		// }

		directionMap0 := move.CreateDirectionMap(0)
		directionMap5 := move.CreateDirectionMap(5)

		ranking := move.EstimateRanking()
		log.Println("ranking: ", ranking)
		var target int
		if ranking[0].player == 0 {
			target = ranking[1].player
		} else if ranking[1].player == 0 {
			target = ranking[0].player
		} else {
			target = ranking[1].player
		}
		log.Println("target: ", target)

		predictions0 := make([]Prediction, 0, 4)
		// 4方向で移動した場合を全部シミュレーションする
		for d := 0; d < 4; d++ {
			predictions0 = append(predictions0, CreatePrediction(move, 0, d, target, directionMap0))
		}

		sort.Slice(predictions0, func(i, j int) bool {
			return predictions0[i].LessThan(&predictions0[j])
		})

		log.Println("predictions0: ", predictions0)

		predictions5 := make([]Prediction, 0, 4)
		// 4方向で移動した場合を全部シミュレーションする
		for d := 0; d < 4; d++ {
			predictions5 = append(predictions5, CreatePrediction(move, 5, d, target, directionMap5))
		}

		sort.Slice(predictions5, func(i, j int) bool {
			return predictions5[i].LessThan(&predictions5[j])
		})

		log.Println("predictions5: ", predictions5)

		idx_0 := 0
		idx_5 := 0

		// pendulum 2
		if len(agentLog) > 2 {
			previousPos := agentLog[len(agentLog)-1][0]
			pos := predictions0[0].pos
			if IsSamePos(pos, previousPos) && IsSameState(move.Field[pos[0]][pos[1]][pos[2]], fieldLog[len(fieldLog)-2][pos[0]][pos[1]][pos[2]]) {
				log.Println("pendulum 0 for 2")
				idx_0 = 1
			}
		}
		if len(agentLog) > 2 {
			previousPos := agentLog[len(agentLog)-1][5]
			pos := predictions5[0].pos
			if IsSamePos(pos, previousPos) && IsSameState(move.Field[pos[0]][pos[1]][pos[2]], fieldLog[len(fieldLog)-2][pos[0]][pos[1]][pos[2]]) {
				log.Println("pendulum 5 for 2")
				idx_5 = 1
			}
		}
		// pendulum 4
		if len(agentLog) > 4 && idx_0 == 0 {
			previousPos := agentLog[len(agentLog)-3][0]
			pos := predictions0[0].pos
			if IsSamePos(pos, previousPos) && IsSameState(move.Field[pos[0]][pos[1]][pos[2]], fieldLog[len(fieldLog)-4][pos[0]][pos[1]][pos[2]]) {
				log.Println("pendulum 0 for 4")
				idx_0 = 1
			}
		}
		if len(agentLog) > 4 && idx_5 == 0 {
			previousPos := agentLog[len(agentLog)-3][5]
			pos := predictions5[0].pos
			if IsSamePos(pos, previousPos) && IsSameState(move.Field[pos[0]][pos[1]][pos[2]], fieldLog[len(fieldLog)-4][pos[0]][pos[1]][pos[2]]) {
				log.Println("pendulum 5 for 4")
				idx_5 = 1
			}
		}

		if idx_0 == 0 {
			minPotential := predictions0[0].potential
			minPotentialIdx := 0
			maxPotential := 0
			for i, p := range predictions0 {
				if p.potential <= minPotential {
					minPotential = p.potential
					minPotentialIdx = i
				}
				if p.potential >= maxPotential {
					maxPotential = p.potential
				}
			}
			if predictions0[0].shortTermPrediction.priority > SelfHalfMayConflict && minPotentialIdx == 0 && maxPotential-minPotential > 700 {
				log.Println("potential 0")
				idx_0 = 1
			}
		}

		if idx_5 == 0 {
			minPotential := predictions5[0].potential
			minPotentialIdx := 0
			maxPotential := 0
			for i, p := range predictions5 {
				if p.potential <= minPotential {
					minPotential = p.potential
					minPotentialIdx = i
				}
				if p.potential >= maxPotential {
					maxPotential = p.potential
				}
			}
			if predictions5[0].shortTermPrediction.priority > SelfHalfMayConflict && minPotentialIdx == 0 && maxPotential-minPotential > 700 {
				log.Println("potential 5")
				idx_5 = 1
			}
		}

		if IsSamePos(move.Agent[0], move.Agent[5]) {
			log.Println("0 and 5 is the same")
			idx_5 += 1
		}

		if IsSamePos(predictions0[idx_0].pos, predictions5[idx_5].pos) {
			log.Println("prediction is the same pos")
			idx_0 += 1
		}

		nextDir0 = strconv.Itoa(predictions0[idx_0].rotation)
		nextDir5 = strconv.Itoa(predictions5[idx_5].rotation)

		if move.Turn > 146 {
			specialDone := false
			if move.Special[0] > 0 {
				special := NewSpecialPredictionStraight(move, move.Agent[0], 0, target)
				for d := 1; d < 4; d++ {
					special2 := NewSpecialPredictionStraight(move, move.Agent[0], d, target)
					if !special2.ShouldSkip() && special2.Point() > special.Point() {
						special = special2
					}
				}
				if !special.ShouldSkip() {
					log.Println("special 0")
					nextDir0 = special.ApiCall()
					specialDone = true
				}
			}
			if !specialDone && move.Special[5] > 0 {
				special := NewSpecialPredictionStraight(move, move.Agent[5], 0, target)
				for d := 1; d < 4; d++ {
					special2 := NewSpecialPredictionStraight(move, move.Agent[5], d, target)
					if !special2.ShouldSkip() && special2.Point() > special.Point() {
						special = special2
					}
				}
				if !special.ShouldSkip() {
					log.Println("special 5")
					nextDir5 = special.ApiCall()
					specialDone = true
				}
			}
		}

		log.Println("next dir0: ", nextDir0)
		log.Println("next dir5: ", nextDir5)

		t := time.Now()
		elapsed := t.Sub(start)
		log.Println("turn: ", move.Turn, ", elapsed: ", elapsed)

		fieldLog = append(fieldLog, move.Field)
		agentLog = append(agentLog, move.Agent)
	}
}

func main() {
	log.Println("Version 10")
	// maps := CreateDirectionLengthMap([]int{0, 1, 2, 0})
	// for i := 0; i < 4; i++ {
	// 	fmt.Println("map ", i, " ---------")
	// 	printMap(maps[i])
	// }
	NewProgram().solve()
}
