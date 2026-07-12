// =============================================================================
// example-high-throughput.ts (AssemblyScript)
//
// 高帯域アラートルール:
// throughput の TxBytes + RxBytes 合計が閾値を超えたらアラートを発行する
//
// AssemblyScript でコンパイル:
//   npm init -y
//   npm install assemblyscript
//   npx asc example-high-throughput.ts -o example-high-throughput.wasm \
//     --exportRuntime --initialMemory 1
//
// 実行:
//   kubectl detective rule --rule example-high-throughput.wasm
// =============================================================================

// ---- ホスト関数 (detective が提供) ----
@external("env", "host_log")
declare function host_log(ptr: i32, len: i32): void;

@external("env", "host_alert")
declare function host_alert(ptr: i32, len: i32): void;

// ---- 文字列定数 ----
const NAME: string = "high-throughput";
const DESC: string = "Alert when total bytes exceed threshold";

// 閾値: 10MB (10 * 1024 * 1024)
const THRESHOLD: u64 = 10 * 1024 * 1024;

// ---- ルールAPI ----

export function rule_name(): i32 {
  return changetype<i32>(NAME);
}

export function rule_description(): i32 {
  return changetype<i32>(DESC);
}

export function rule_init(): i32 {
  return 0; // 成功
}

// ---- JSON スキャナー (簡易版) ----
// AssemblyScript には標準の JSON パーサーがないため、
// "tx_bytes" キーの値をバイトスキャンで取得する。

function readU64(json: string, key: string): u64 {
  let keyPos = json.indexOf(key);
  if (keyPos < 0) return 0;

  // ":" を探す
  let colonPos = json.indexOf(":", keyPos + key.length);
  if (colonPos < 0) return 0;

  // 数値の先頭
  let numStart = colonPos + 1;
  while (numStart < json.length && json.charCodeAt(numStart) == 32) {
    numStart++; // スキップ
  }

  // 数値を読む
  let result: u64 = 0;
  let pos = numStart;
  while (pos < json.length) {
    let ch = json.charCodeAt(pos);
    if (ch >= 48 && ch <= 57) { // '0'-'9'
      result = result * 10 + (ch - 48) as u64;
      pos++;
    } else {
      break;
    }
  }
  return result;
}

export function rule_evaluate(ctx_ptr: i32, ctx_len: i32): i32 {
  // コンテキストを文字列として読み取る
  let ctx = String.UTF8.decodeUnsafe(ctx_ptr, ctx_len);

  // total throughput を集計 (簡易: "tx_bytes" と "rx_bytes" を全て探す)
  let totalTx: u64 = 0;
  let totalRx: u64 = 0;

  // 全throughputsエントリをスキャン
  let searchFrom = 0;
  while (true) {
    let txPos = ctx.indexOf("tx_bytes", searchFrom);
    if (txPos < 0) break;
    totalTx += readU64(ctx.substring(txPos), "tx_bytes");
    searchFrom = txPos + 8;
  }

  searchFrom = 0;
  while (true) {
    let rxPos = ctx.indexOf("rx_bytes", searchFrom);
    if (rxPos < 0) break;
    totalRx += readU64(ctx.substring(rxPos), "rx_bytes");
    searchFrom = rxPos + 8;
  }

  let total = totalTx + totalRx;

  // 閾値チェック
  if (total > THRESHOLD) {
    let msg = "High throughput detected: " + total.toString() + " bytes (threshold: " + THRESHOLD.toString() + ")";
    host_alert(changetype<i32>(msg), msg.lengthUTF8);
    return 1; // ALERT
  }

  return 0; // PASS
}
