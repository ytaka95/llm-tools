# 構築方法

## 前提

- goコマンドをインストール済み
    - version 1.24.1 で動作確認
- Google Cloud のAPIキーを環境変数 `API_KEY_GOOGLE` に設定済み

## 依存関係のインストール

```sh
go mod tidy
```

## ビルド

```sh
go build -o english-translator
```

## 実行

```sh
./english-translator
```
