# 構築方法

## 前提

- goコマンドをインストール済み
    - version 1.24.1 で動作確認
- Geminiへアクセスする方法として以下のいずれか
    - Google Cloud のAPIキーを環境変数 `API_KEY_GOOGLE` に設定済み
    - gcloudコマンドでVertexAIを使うプロジェクトへアクセス可能
        1. gcloudコマンドをインストール
        1. ユーザーアカウントで認証: `gcloud auth application-default login`

## 依存関係のインストール

```sh
go mod tidy
```

## ビルド

```sh
# simple build
go build -o llm-english-translator

# for Production
go build -ldflags "-s -w" -o ~/.local/myapps/bin/llm-english-translator .
```

## 実行

```sh
./llm-english-translator --task translate [翻訳したい日本語テキスト]
```

タスクを指定する場合

```sh
# 翻訳
./llm-english-translator --task translate --model gemini-2.5-flash "翻訳したい日本語テキスト"

# 技術的な質問に回答
./llm-english-translator --task tech-qa "GoでJSONを整形するには？"

# Gemini 3 の思考レベルを指定
./llm-english-translator --task translate --model gemini-3-flash-preview --think-level medium "翻訳したい日本語テキスト"

# gemini-3-pro* は low / high のみ指定可能
./llm-english-translator --task translate --model gemini-3-pro --think-level low "翻訳したい日本語テキスト"
```

ヘルプ表示

```sh
./llm-english-translator -help
```

初回起動時に対話式のセットアップが始まります。設定ファイルは `~/.config/llm-english-translator/settings.json` に保存されます。
