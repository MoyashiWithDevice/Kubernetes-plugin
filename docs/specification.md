# kubectl-detective 仕様書

## 1. 概要

kubectl-detective は Kubernetes クラスタ内の通信を eBPF により収集し、

* Service Map の可視化
* 通信量の測定
* TCP Retransmission の検出
* DNS レイテンシの分析
* 異常検知
* 統計情報のエクスポート

を行う診断プラットフォームである。

主な目的は、

「なぜ通信が遅いのか」
「どの Pod がどこと通信しているのか」
「どこでパケットロスが発生しているのか」

を容易に把握できるようにすることである。

---

# 2. アーキテクチャ

```text
kubectl plugin
       ↑
REST API
       ↑
Aggregator Deployment
       ↑
DaemonSet Agent
       ↑
eBPF Programs
```

---

# 3. コンポーネント

## Agent

Node ごとに DaemonSet として配置する。

役割

* eBPF イベント収集
* RingBuffer 読み出し
* Flow 集計
* Aggregator への送信

---

## Aggregator

Deployment として動作。

役割

* Pod と Service の対応付け
* Flow の集約
* 時系列統計保持
* API 提供
* エクスポート処理

---

## kubectl plugin

CLI ツール。

役割

* 情報表示
* グラフ生成
* 統計出力

---

# 4. 収集対象

## TCP Flow

取得項目

* Source Pod
* Destination Pod
* Source IP
* Destination IP
* Source Port
* Destination Port
* Packet Count
* Byte Count

---

## Throughput

算出項目

* TX Mbps
* RX Mbps
* Peak Mbps

---

## TCP Retransmission

取得項目

* retransmission count
* retransmission rate
* affected flow

---

## RTT

取得項目

* average RTT
* p95 RTT
* p99 RTT

---

## DNS

取得項目

* Query
* Response Time
* NXDOMAIN Count

---

# 5. Service Map

ノード

* Namespace
* Service
* Pod

エッジ情報

* Throughput
* Packet Count
* Retransmission Count
* Average RTT

出力形式

* Mermaid
* Graphviz
* JSON

例

frontend → api → redis

api → postgres

---

# 6. CLI

## map

通信マップ表示

```bash
kubectl detective map
```

---

## top

通信統計表示

```bash
kubectl detective top
```

---

## retrans

再送ランキング

```bash
kubectl detective retrans
```

---

## dns

DNS分析

```bash
kubectl detective dns
```

---

## why

原因分析

```bash
kubectl detective why api
```

---

## export

統計出力

```bash
kubectl detective export
```

---

# 7. 出力フォーマット

## CSV

```csv
timestamp,source,destination,mbps,retrans,rtt
```

用途

Excel分析

---

## JSON

```json
{
  "source":"frontend",
  "destination":"api",
  "throughput":15.2,
  "retransmissions":4
}
```

用途

他システム連携

---

## YAML

設定バックアップ

---

## Mermaid

サービス依存関係

---

## Graphviz

画像生成

---

## HTML Report

日次レポート

内容

* Top Talkers
* Retransmission Ranking
* RTT Ranking
* DNS Statistics
* Service Map

---

# 8. REST API

GET /flows

Flow 一覧

GET /services

Service 一覧

GET /top

通信量ランキング

GET /retrans

再送統計

GET /dns

DNS統計

GET /map

Service Map

GET /export

統計ファイル生成

---

# 9. WASM Rule Engine

実行環境

wazero

目的

ユーザー定義ルール実行

例

DNS latency > 100ms

Retransmission rate > 5%

RTT > 500ms

Throughput > 1Gbps

アクション

* Warning
* Slack
* Webhook
* Event 作成

---

# 10. 将来的な拡張

## HTTP L7解析

* Path
* Method
* Latency

---

## OpenTelemetry連携

---

## Prometheus Exporter

---

## Grafana Dashboard

---

## CRD

NetworkInsight

---

## AI異常検知

---

## Web UI

* Cytoscape
* D3.js

---

## pcap生成

異常発生時のパケット保存

---

## Cluster間 Service Map

マルチクラスタ対応

---

# 使用技術

言語

Go

ライブラリ

* cilium/ebpf
* client-go
* cobra
* wazero
* grpc
* protobuf

デプロイ

* DaemonSet
* Deployment

対象

Kubernetes 1.30+

Linux kernel 5.15+

```
```
