# kubectl-detective 開発ロードマップ

# Phase 0 : 開発環境

目的

開発可能な状態を作る。

タスク

* Goプロジェクト作成
* Cobra導入
* Makefile作成
* kindクラスタ構築
* テスト用Deployment作成

完了条件

```bash
kubectl detective version
```

が動作する。

---

# Phase 1 : Pod情報取得

目的

Kubernetes APIとの接続。

タスク

* client-go導入
* kubeconfig読み込み
* Pod一覧取得
* Service一覧取得
* EndpointSlice取得

CLI

```bash
kubectl detective pods
kubectl detective services
```

完了条件

PodとServiceの対応が分かる。

---

# Phase 2 : Flow収集

目的

通信情報取得。

タスク

* cilium/ebpf導入
* RingBuffer構築
* TCP connect監視
* source IP取得
* destination IP取得

出力

```text
10.0.1.10 → 10.0.1.15
```

完了条件

通信をリアルタイム表示できる。

---

# Phase 3 : Pod名解決

目的

IPからPod名へ変換。

タスク

* Informer追加
* Pod cache作成
* IP→Pod対応表作成

出力

```text
frontend → api
```

完了条件

Pod間通信が見える。

---

# Phase 4 : Service Map

目的

依存関係可視化。

タスク

* Graph構造作成
* Edge集約
* Mermaid出力

CLI

```bash
kubectl detective map
```

出力

frontend → api → redis

完了条件

Mermaid生成。

---

# Phase 5 : Throughput

目的

帯域計測。

タスク

* byte counter追加
* Mbps計算
* Top Talker集計

CLI

```bash
kubectl detective top
```

完了条件

通信量ランキング表示。

---

# Phase 6 : Retransmission

目的

品質監視。

タスク

* tcp_retransmit_skb追跡
* Event集計
* retransmission rate計算

CLI

```bash
kubectl detective retrans
```

完了条件

再送ランキング表示。

---

# Phase 7 : RTT

目的

遅延監視。

タスク

* RTT取得
* p95計算
* p99計算

CLI

```bash
kubectl detective latency
```

完了条件

Pod間遅延表示。

---

# Phase 8 : DNS分析

目的

CoreDNS問題解析。

タスク

* UDP53監視
* Query取得
* latency集計

CLI

```bash
kubectl detective dns
```

完了条件

DNS統計表示。

---

# Phase 9 : Export

目的

障害解析。

タスク

* CSV writer
* JSON writer
* Mermaid writer
* HTML report

CLI

```bash
kubectl detective export
```

完了条件

ファイル生成。

---

# Phase 10 : DaemonSet化

目的

全ノード監視。

タスク

* Agent作成
* Aggregator作成
* gRPC追加

完了条件

クラスタ全体を監視。

---

# Phase 11 : WASM Rule Engine

目的

拡張性。

タスク

* wazero導入
* plugin API設計
* rule.wasmロード

完了条件

ユーザー定義ルール実行。

---

# Phase 12 : Web UI

目的

可視化。

タスク

* REST API
* Cytoscape
* D3.js

完了条件

ブラウザ表示。

---

# Phase 13 : OSS化

目的

公開。

タスク

* README
* Helm Chart
* GitHub Actions
* Release

```
```
