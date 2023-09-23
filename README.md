# 天下一 Game Battle Contest 2023

- [公式サイト](https://tenka1.klab.jp/2023/)
- [YouTube配信](https://www.youtube.com/watch?v=PxG2794Ujfg)
- [ポータルサイト](https://gbc2023.tenka1.klab.jp/portal/index.html)

## ドキュメント

- [問題概要およびAPI仕様](problem.md)
- [チュートリアル](tutorial.md)
- [Runnerの使い方](runner.md)
- [ポータルの使い方](portal.md)
- [ビジュアライザの使い方](visualizer.md)

## サンプルコード

- [Go](go)
  - go1.21.1 で動作確認
- [Python](py)
  - python 3.8.10, python 3.11.2 で動作確認
- [C#](cs)
  - dotnet 6.0.406 で動作確認
- [Rust](rust)
  - cargo 1.66.1 で動作確認
- [C++(libcurl)](cpp) 通信にライブラリを使用
  - g++ 9.4.0 で動作確認
- [C++(Python)](cpp_and_python) 通信にPythonを使用
  - python 3.8.10, g++ 9.4.0 で動作確認


動作確認環境はいずれも Ubuntu 20.04 LTS

## ルール

- コンテスト期間
  - 2023年9月23日(土・祝)
    - 予選リーグ: 14:00～18:00
    - 決勝リーグ: 18:00～18:20
    - ※予選リーグ終了後、上位8名による決勝リーグを開催
- 参加資格
  - 学生、社会人問わず、どなたでも参加可能です。他人と協力せず、個人で取り組んでください。
- 使用可能言語
  - 言語の制限はありません。ただしHTTPSによる通信ができる必要があります。
- SNS等の利用について
  - 本コンテスト開催中にSNS等にコンテスト問題について言及して頂いて構いませんが、ソースコードを公開するなどの直接的なネタバレ行為はお控えください。
ハッシュタグ: #klabtenka1

## その他

- [ギフトカード抽選プログラム](lottery)
  - 抽選対象は join API で開始したゲームに一度以上 move API を実行した参加者です
