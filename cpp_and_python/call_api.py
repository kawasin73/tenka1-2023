import os
import subprocess
import sys
import time
import requests

# ゲームサーバのアドレス / トークン
GAME_SERVER = os.getenv('GAME_SERVER', 'https://gbc2023.tenka1.klab.jp')
TOKEN = os.getenv('TOKEN', 'YOUR_TOKEN')

p = subprocess.Popen(sys.argv[1:], stdin=subprocess.PIPE, stdout=subprocess.PIPE)

session = requests.Session()


# ゲームサーバのAPIを叩く
def call_api(x: str) -> dict:
    url = f'{GAME_SERVER}{x}'
    for _ in range(5):
        print(url, flush=True)
        try:
            response = session.get(url)

            if response.status_code == 200:
                return response.json()

            # 5xxエラーまたはRequestExceptionの際は100ms空けて5回までリトライする
            if 500 <= response.status_code < 600:
                print(response.status_code)
                time.sleep(0.1)
                continue

            raise Exception('Api Error status_code:{}'.format(response.status_code))

        # 5xxエラーまたはRequestExceptionの際は100ms空けて5回までリトライする
        except requests.RequestException as e:
            print(e)
            time.sleep(0.1)
            continue
    raise Exception('Api Error')


# 指定したmode, delayで練習試合開始APIを呼ぶ
def call_start(mode: int, delay: int):
    return call_api(f"/api/start/{TOKEN}/{mode}/{delay}")


# dir方向に移動するように移動APIを呼ぶ
def call_move(game_id: int, dir0: str, dir5: str):
    return call_api(f"/api/move/{TOKEN}/{game_id}/{dir0}/{dir5}")


# game_idを取得する
# 環境変数で指定されていない場合は練習試合のgame_idを返す
def get_game_id() -> int:
    # 環境変数にGAME_IDが設定されている場合これを優先する
    if os.getenv('GAME_ID'):
        return int(os.getenv('GAME_ID'))

    # start APIを呼び出し練習試合のgame_idを取得する
    start = call_start(0, 0)
    if start['status'] == 'ok' or start['status'] == 'started':
        return start['game_id']

    raise Exception(f'Start Api Error : {start}')


class Program:
    @staticmethod
    def solve():
        game_id = get_game_id()
        while True:
            line = p.stdout.readline()
            if not line:
                break
            next_dir0, next_dir5 = line.decode().rstrip('\r\n').split(' ')

            # 移動APIを呼ぶ
            move = call_move(game_id, next_dir0, next_dir5)
            print(f"status = {move['status']}", file=sys.stderr, flush=True)
            if move['status'] == "already_moved":
                continue
            elif move['status'] != 'ok':
                break

            p.stdin.write(f'{move["now"]} {move["turn"]}\n'.encode())
            assert len(move['move']) == 6
            p.stdin.write((' '.join(str(x) for x in move['move']) + '\n').encode())
            assert len(move['score']) == 3
            p.stdin.write((' '.join(str(x) for x in move['score']) + '\n').encode())
            assert len(move['field']) == 6
            for face in move['field']:
                assert len(face) == 5
                for row in face:
                    assert len(row) == 5
                    for cell in row:
                        assert len(cell) == 2
                        p.stdin.write(f'{cell[0]} {cell[1]}\n'.encode())
            assert len(move['agent']) == 6
            for agent in move['agent']:
                assert len(agent) == 4
                p.stdin.write((' '.join(str(x) for x in agent) + '\n').encode())
            assert len(move['special']) == 6
            p.stdin.write((' '.join(str(x) for x in move['special']) + '\n').encode())
            p.stdin.flush()


if __name__ == "__main__":
    Program().solve()
