;; =============================================================================
;; example-retrans-alert.wat
;;
;; 再送(retransmission)アラートルール例:
;; JSONコンテキスト内に "retransmissions" キーが含まれる場合にアラートを発行する
;;
;; 実際のルールではJSONパースを行い count 値を閾値と比較するが、
;; WATのみではJSONパースが複雑なため、ここでは文字列スキャンでデモする。
;;
;; ビルド方法:
;;   wat2wasm example-retrans-alert.wat -o example-retrans-alert.wasm
;;
;; 実行:
;;   kubectl detective rule --rule example-retrans-alert.wasm
;; =============================================================================

(module
  ;; ---------------------------------------------------------------
  ;; ホスト関数
  ;; ---------------------------------------------------------------
  (import "env" "host_log"    (func $host_log    (param i32 i32)))
  (import "env" "host_alert"  (func $host_alert  (param i32 i32)))

  (memory (export "memory") 1)

  ;; 文字列定数
  (data (i32.const 0)   "retrans-alert\00")                   ;; 0..14
  (data (i32.const 15)  "Alert on retransmissions found\00")  ;; 15..46
  (data (i32.const 47)  "[rule] retransmissions detected!\00") ;; 47..80
  (data (i32.const 81)  "retransmissions\00")                  ;; 81..97 (検索パターン)

  ;; rule_name
  (func (export "rule_name") (result i32)
    i32.const 0
  )

  ;; rule_description
  (func (export "rule_description") (result i32)
    i32.const 15
  )

  ;; rule_init
  (func (export "rule_init") (result i32)
    i32.const 0
  )

  ;; ---------------------------------------------------------------
  ;; rule_evaluate:
  ;;   JSON 内をスキャンし "retransmissions" という文字列を探す
  ;;   見つかればホストにアラートを送信し ALERT(1) を返す
  ;; ---------------------------------------------------------------
  (func (export "rule_evaluate") (param $ctx_ptr i32) (param $ctx_len i32) (result i32)
    (local $i i32)
    (local $end i32)
    (local $match i32)
    (local $pat_ptr i32)
    (local $pat_len i32)
    (local $ch i32)

    ;; パターン "retransmissions" のアドレスと長さ
    (local.set $pat_ptr (i32.const 81))
    (local.set $pat_len (i32.const 16))  ;; "retransmissions" = 16 bytes

    ;; スキャン範囲
    (local.set $i (local.get $ctx_ptr))
    (local.set $end (i32.add (local.get $ctx_ptr) (local.get $ctx_len)))

    ;; 外部ループ: コンテキスト全体をスキャン
    (block $break
      (loop $scan
        ;; 範囲チェック
        (br_if $break (i32.ge_u (local.get $i) (local.get $end)))

        ;; パターン先頭バイトと比較
        (if (i32.eq
              (i32.load8_u (local.get $i))
              (i32.load8_u (local.get $pat_ptr)))
          (then
            ;; パターンマッチ確認
            (local.set $match (i32.const 1))
            (block $pat_break
              (loop $pat_check
                ;; パターン末尾に達したらマッチ成立
                (br_if $pat_break
                  (i32.ge_u (local.get $match) (local.get $pat_len)))

                ;; 範囲チェック
                (br_if $pat_break
                  (i32.ge_u
                    (i32.add (local.get $i) (local.get $match))
                    (local.get $end)))

                ;; バイト比較
                (if (i32.ne
                      (i32.load8_u (i32.add (local.get $i) (local.get $match)))
                      (i32.load8_u (i32.add (local.get $pat_ptr) (local.get $match))))
                  (then
                    (local.set $match (i32.const 0))
                    (br $pat_break)
                  )
                )

                (local.set $match (i32.add (local.get $match) (i32.const 1)))
                (br $pat_check)
              )
            )

            ;; マッチしたらアラートを発行
            (if (i32.gt_s (local.get $match) (i32.const 0))
              (then
                ;; host_alert にメッセージを送信
                (call $host_alert (i32.const 47) (i32.const 33))
                ;; host_log にも記録
                (call $host_log (i32.const 47) (i32.const 33))
                ;; ALERT を返す
                (return (i32.const 1))
              )
            )
          )
        )

        (local.set $i (i32.add (local.get $i) (i32.const 1)))
        (br $scan)
      )
    )

    ;; パターン未発見 → PASS
    i32.const 0
  )
)
