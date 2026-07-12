// =============================================================================
// lib.rs (Rust + wasm-bindgen)
//
// RTT 遅延アラートルール:
// Pod 間の平均遅延 (avg_us) が閾値を超えたらアラートを発行する
//
// ビルド方法:
//   rustup target add wasm32-unknown-unknown
//   cargo build --target wasm32-unknown-unknown --release
//
// または wasm-pack を使う場合:
//   wasm-pack build --target no-modules
//
// 実行:
//   kubectl detective rule --rule target/wasm32-unknown-unknown/release/my_rule.wasm
// =============================================================================

use std::os::raw::c_void;

// ---- ホスト関数 (detective が提供) ----
extern "C" {
    fn host_log(ptr: *const u8, len: usize);
    fn host_alert(ptr: *const u8, len: usize);
}

// ---- ヘルパー関数 ----

unsafe fn log_message(msg: &str) {
    host_log(msg.as_ptr(), msg.len());
}

unsafe fn send_alert(msg: &str) {
    host_alert(msg.as_ptr(), msg.len());
}

/// JSON内から数値を簡易的に取得する
/// "key": 12345 のようなパターンを探す
fn find_json_number(json: &str, key: &str) -> Option<u64> {
    let pattern = format!("\"{}\":", key);
    let pos = json.find(&pattern)?;
    let after = &json[pos + pattern.len()..];

    // 先頭の空白をスキップ
    let trimmed = after.trim_start();

    // 数値を読む
    let end = trimmed.find(|c: char| !c.is_ascii_digit()).unwrap_or(trimmed.len());
    if end == 0 {
        return None;
    }
    trimmed[..end].parse::<u64>().ok()
}

// ---- ルール定数 ----

const RTT_THRESHOLD_US: u64 = 100_000; // 100ms = 100,000μs

// ---- WASM エクスポート ----

#[no_mangle]
pub extern "C" fn rule_name() -> *const u8 {
    b"high-latency\0".as_ptr()
}

#[no_mangle]
pub extern "C" fn rule_description() -> *const u8 {
    b"Alert when average RTT exceeds threshold\0".as_ptr()
}

#[no_mangle]
pub extern "C" fn rule_init() -> i32 {
    0 // 成功
}

/// rule_evaluate: コンテキスト JSON を解析し、RTTが閾値を超えたらアラート
///
/// JSON構造 (例):
/// ```json
/// {
///   "rtt": [
///     {"src_ip":"10.0.0.1","dst_ip":"10.0.0.2","avg_us":50000,"count":100},
///     {"src_ip":"10.0.0.3","dst_ip":"10.0.0.4","avg_us":200000,"count":50}
///   ]
/// }
/// ```
#[no_mangle]
pub extern "C" fn rule_evaluate(ctx_ptr: *const u8, ctx_len: usize) -> i32 {
    let ctx = unsafe {
        let slice = std::slice::from_raw_parts(ctx_ptr, ctx_len);
        std::str::from_utf8_unchecked(slice)
    };

    // 簡易スキャン: "avg_us" の値を全て取得して最大値を求める
    let mut max_avg: u64 = 0;
    let mut search_start = 0;

    while let Some(pos) = ctx[search_start..].find("\"avg_us\":") {
        let abs_pos = search_start + pos + 9; // "avg_us": の後
        let after = &ctx[abs_pos..];
        let trimmed = after.trim_start();
        let end = trimmed.find(|c: char| !c.is_ascii_digit()).unwrap_or(trimmed.len());
        if end > 0 {
            if let Ok(val) = trimmed[..end].parse::<u64>() {
                if val > max_avg {
                    max_avg = val;
                }
            }
        }
        search_start = abs_pos + end;
    }

    // 閾値チェック
    if max_avg > RTT_THRESHOLD_US {
        let msg = format!(
            "High latency detected: max avg_us = {} (threshold: {})",
            max_avg, RTT_THRESHOLD_US
        );
        unsafe {
            send_alert(&msg);
        }
        1 // ALERT
    } else {
        0 // PASS
    }
}
