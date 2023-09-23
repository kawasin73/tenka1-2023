using System.Text.Json;
using System.Text.Json.Serialization;

public partial class GameLogic
{
    public Cell GetCell(int i, int j, int k)
    {
        return _field[FieldIdx(i,j,k)];
    }

    private static int[] ArrayCopy(int[] a)
    {
        var r = new int[a.Length];
        Array.Copy(a, r, a.Length);
        return r;
    }

    internal GameLogic(MoveResponse move)
    {
        _field = new Cell[6*N*N];
        _agents = new Agent[6];
        _turn = move.Turn;
        _move = ArrayCopy(move.Move);
        _score = ArrayCopy(move.Score);
        _area = new int[3];
        _special = ArrayCopy(move.Special);
        for (var i = 0; i < 6; i++)
        {
            for (var j = 0; j < N; j++)
            {
                for (var k = 0; k < N; k++)
                {
                    var owner = move.Field[i][j][k][0];
                    _field[FieldIdx(i, j, k)] = new Cell { Owner = owner, Val = move.Field[i][j][k][1] };
                    if (owner >= 0)
                    {
                        ++ _area[owner];
                    }
                }
            }
        }

        for (var i = 0; i < 6; i++)
        {
            _agents[i] = new Agent { I = move.Agent[i][0], J = move.Agent[i][1], K = move.Agent[i][2], D = move.Agent[i][3] };
        }
    }
}

internal class Program
{
    private static readonly HttpClient Client = new HttpClient();

    private readonly string _gameServer;
    private readonly string _token;

    private const int N = 5;
    private static Random rand = new Random();

    // ゲームサーバのAPIを叩く
    private async Task<byte[]> CallApi(string x)
    {
        var url = $"{_gameServer}{x}";
        // 5xxエラーまたはHttpRequestExceptionの際は100ms空けて5回までリトライする
        for (var i = 0; i < 5; i++)
        {
            Console.WriteLine(url);
            try
            {
                var res = await Client.GetAsync(url);
                if ((int)res.StatusCode == 200)
                {
                    return await res.Content.ReadAsByteArrayAsync();
                }

                if (500 <= (int)res.StatusCode && (int)res.StatusCode < 600)
                {
                    Console.WriteLine($"{res.StatusCode}");
                    Thread.Sleep(100);
                    continue;
                }

                throw new Exception($"Api Error status_code:{res.StatusCode}");
            }
            catch (HttpRequestException e)
            {
                Console.WriteLine($"{e.Message}");
                Thread.Sleep(100);
            }
        }
        throw new Exception("Api Error");
    }

    // 指定したmode, delayで練習試合開始APIを呼ぶ
    async Task<StartResponse> CallStart(int mode, int delay)
    {
        var json = await CallApi($"/api/start/{_token}/{mode}/{delay}");
        return JsonSerializer.Deserialize<StartResponse>(json);
    }

    // dir方向に移動するように移動APIを呼ぶ
    async Task<MoveResponse> CallMove(int gameId, string dir0, string dir5)
    {
        var json = await CallApi($"/api/move/{_token}/{gameId}/{dir0}/{dir5}");
        return JsonSerializer.Deserialize<MoveResponse>(json);
    }

    // game_idを取得する
    // 環境変数で指定されていない場合は練習試合のgame_idを返す
    async Task<int> GetGameId()
    {
        // 環境変数にGAME_IDが設定されている場合これを優先する
        var envGameId = Environment.GetEnvironmentVariable("GAME_ID");
        if (envGameId != null)
        {
            return int.Parse(envGameId);
        }

        // start APIを呼び出し練習試合のgame_idを取得する
        var start = await CallStart(0, 0);
        if (start.Status == "ok" || start.Status == "started")
        {
            return start.GameId;
        }

        throw new Exception($"Start Api Error : {start.Status}");
    }

    private static string UseRandomSpecial(string nextDir)
    {
        // 50%で直進の必殺技を使用
        if (rand.Next(2) == 0)
        {
            return nextDir + "s";
        }
        // 50%でランダムな場所に瞬間移動
        var i = rand.Next(6);
        var j = rand.Next(5);
        var k = rand.Next(5);
        return $"{i}-{j}-{k}";
    }

    // BOTのメイン処理
    private async Task Solve()
    {
        var gameId = await GetGameId();
        var nextDir0 = rand.Next(4).ToString();
        var nextDir5 = rand.Next(4).ToString();
        for (;;)
        {
            // 移動APIを呼ぶ
            var move = await CallMove(gameId, nextDir0, nextDir5);
            Console.WriteLine($"status = {move.Status}");
            if (move.Status == "already_moved")
            {
                continue;
            }
            else if (move.Status != "ok")
            {
                break;
            }
            Console.WriteLine($"turn = {move.Turn}");
            Console.WriteLine($"score = {move.Score[0]} {move.Score[1]} {move.Score[2]}");
            // 4方向で移動した場合を全部シミュレーションする
            var bestC = -1;
            var bestD = new List<(int, int)>();
            for (var d0 = 0; d0 < 4; d0++)
            {
                for (var d5 = 0; d5 < 4; d5++)
                {
                    var m = new GameLogic(move);
                    m.Progress(0, new[] { d0, -1, -1, -1, -1, d5 });
                    // 自身のエージェントで塗られているマス数をカウントする
                    var c = 0;
                    for (var i = 0; i < 6; i++)
                    {
                        for (var j = 0; j < N; j++)
                        {
                            for (var k = 0; k < N; k++)
                            {
                                if (m.GetCell(i, j, k).Owner == 0) c++;
                            }
                        }
                    }

                    // 最も多くのマスを自身のエージェントで塗れる移動方向のリストを保持する
                    if (c > bestC)
                    {
                        bestC = c;
                        bestD.Clear();
                        bestD.Add((d0, d5));
                    }
                    else if (c == bestC)
                    {
                        bestD.Add((d0, d5));
                    }
                }
            }

            // 最も多くのマスを自身のエージェントで塗れる移動方向のリストからランダムで方向を決める
            var best = bestD[rand.Next(bestD.Count)];
            nextDir0 = best.Item1.ToString();
            nextDir5 = best.Item2.ToString();
            // 必殺技を使っていない場合はランダムな確率で使用する
            if (move.Special[0] != 0 && rand.Next(100) < 10)
            {
                nextDir0 = UseRandomSpecial(nextDir0);
            }
            if (move.Special[5] != 0 && rand.Next(100) < 10)
            {
                nextDir5 = UseRandomSpecial(nextDir5);
            }
        }
    }

    private Program()
    {
        _gameServer = Environment.GetEnvironmentVariable("GAME_SERVER") ?? "https://gbc2023.tenka1.klab.jp";
        _token = Environment.GetEnvironmentVariable("TOKEN") ?? "YOUR_TOKEN";
    }

    private static async Task Main(string[] args)
    {
        await new Program().Solve();
    }
}

// 練習試合開始APIのレスポンス用の構造体
internal readonly struct StartResponse
{
    [JsonPropertyName("status")]
    public string Status { get; }
    [JsonPropertyName("game_id")]
    public int GameId { get; }
    [JsonConstructor]
    public StartResponse(string status, int gameId) => (Status, GameId) = (status, gameId);
}

// 移動APIのレスポンス用の構造体
internal readonly struct MoveResponse
{
    [JsonPropertyName("status")]
    public string Status { get; }
    [JsonPropertyName("now")]
    public long Now { get; }
    [JsonPropertyName("turn")]
    public int Turn { get; }
    [JsonPropertyName("move")]
    public int[] Move { get; }
    [JsonPropertyName("score")]
    public int[] Score { get; }
    [JsonPropertyName("field")]
    public int[][][][] Field { get; }
    [JsonPropertyName("agent")]
    public int[][] Agent { get; }
    [JsonPropertyName("special")]
    public int[] Special { get; }
    [JsonConstructor]
    public MoveResponse(string status, long now, int turn, int[] move, int[] score, int[][][][] field, int[][] agent, int[] special)
        => (Status, Now, Turn, Move, Score, Field, Agent, Special) = (status, now, turn, move, score, field, agent, special);
}
