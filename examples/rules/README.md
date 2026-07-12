# WASM Rule Engine

kubectl-detective は WASM (WebAssembly) ベースのルールエンジンをサポートしており、
ユーザー定義ルールを .wasm ファイルとしてロードして実行できます。

## 使い方

```bash
kubectl detective rule --rule <rule.wasm> --duration 30s --interval 5s
```

| オプション | 説明 | デフォルト |
|---|---|---|
| `--rule` | .wasm ファイルのパス (必須) | - |
| `--duration` | 評価の総時間 | 30s |
| `--interval` | 評価間隔 | 5s |
| `--no-headers` | 進捗メッセージを抑制 | false |

## WASM ルールの API コントラクト

ルール WASM モジュールは以下の関数を**エクスポート**する必要があります:

| 関数 | シグネチャ | 説明 |
|---|---|---|
| `rule_name` | `() -> i32` | ルール名の NUL終端文字列へのポインタ |
| `rule_description` | `() -> i32` | 説明の NUL終端文字列へのポインタ |
| `rule_init` | `() -> i32` | 初期化 (0=成功) |
| `rule_evaluate` | `(i32, i32) -> i32` | 評価 (0=PASS, 1=ALERT) |

`rule_evaluate` のパラメータ:
- 1つ目: JSON コンテキストデータの先頭アドレス
- 2つ目: JSON コンテキストデータのバイト長

### ホスト関数 (インポート)

ルール側から呼び出せる関数 (`env` モジュール):

| 関数 | シグネチャ | 説明 |
|---|---|---|
| `host_log` | `(i32, i32) -> ()` | ログ出力 (ptr, len) |
| `host_alert` | `(i32, i32) -> ()` | アラート送信 (ptr, len) |

### メモリ

- `(memory (export "memory") 1)` を含めること
- ホストは `memory.grow` で拡張し、JSON データを書き込む
- `rule_evaluate` が呼ばれる前にデータが書き込まれている

## JSON コンテキスト構造

`rule_evaluate` に渡される JSON の形式:

```json
{
  "throughputs": [
    {
      "src_ip": "10.0.1.10",
      "dst_ip": "10.0.1.15",
      "src_port": 8080,
      "dst_port": 3306,
      "tx_bytes": 1048576,
      "rx_bytes": 524288
    }
  ],
  "retransmissions": [
    {
      "src_ip": "10.0.1.10",
      "dst_ip": "10.0.1.15",
      "src_port": 8080,
      "dst_port": 3306,
      "count": 5
    }
  ],
  "rtt": [
    {
      "src_ip": "10.0.1.10",
      "dst_ip": "10.0.1.15",
      "src_port": 8080,
      "dst_port": 3306,
      "avg_us": 1500,
      "min_us": 500,
      "max_us": 3000,
      "count": 100
    }
  ],
  "flows": [
    {
      "src_ip": "10.0.1.10",
      "dst_ip": "10.0.1.15",
      "src_port": 8080,
      "dst_port": 3306,
      "pid": 12345,
      "comm": "nginx"
    }
  ]
}
```

## 例ファイル

### 1. WAT (WebAssembly Text) で直接書く

```bash
# wabt のインストール (Ubuntu)
apt-get install wabt

# ビルド
wat2wasm example-always-pass.wat -o example-always-pass.wasm
wat2wasm example-retrans-alert.wat -o example-retrans-alert.wasm

# 実行
kubectl detective rule --rule example-always-pass.wasm
```

### 2. AssemblyScript (TypeScript風)

```bash
cd examples/rules
npm init -y
npm install assemblyscript
npx asc example-high-throughput.ts \
  -o example-high-throughput.wasm \
  --exportRuntime --initialMemory 1

kubectl detective rule --rule example-high-throughput.wasm
```

### 3. Rust

```bash
cd examples/rules/rust-rule-example
rustup target add wasm32-unknown-unknown
cargo build --target wasm32-unknown-unknown --release

kubectl detective rule \
  --rule target/wasm32-unknown-unknown/release/high_latency_rule.wasm
```

## 推奨事項

- **言語選択**: AssemblyScript か Rust の使用を推奨。WAT での直接記述は学習用に留める
- **JSON パース**: WAT では JSON パースが困難なため、キーワードスキャンで代替する
- **パフォーマンス**: `rule_evaluate` は短時間で完了させること。重い処理は `rule_init` で行う
- **エラーハンドリング**: `rule_init` で前処理を完了させ、`rule_evaluate` は高速に判断のみ行う
