package rule

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/api"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
)

type Engine struct {
	r      wazero.Runtime
	module api.Module
	name   string
	desc   string
	alerts []string
}

func NewEngine() *Engine {
	return &Engine{}
}

func (e *Engine) Load(ctx context.Context, wasmPath string) error {
	data, err := os.ReadFile(wasmPath)
	if err != nil {
		return fmt.Errorf("read wasm: %w", err)
	}

	r := wazero.NewRuntime(ctx)
	wasi_snapshot_preview1.MustInstantiate(ctx, r)

	_, err = r.NewHostModuleBuilder("env").
		NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
			ptr := uint32(stack[0])
			length := uint32(stack[1])
			buf, ok := m.Memory().Read(ptr, length)
			if !ok {
				return
			}
			log.Printf("[rule] %s", string(buf))
		}), []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, nil).
		Export("host_log").
		NewFunctionBuilder().
		WithGoModuleFunction(api.GoModuleFunc(func(ctx context.Context, m api.Module, stack []uint64) {
			ptr := uint32(stack[0])
			length := uint32(stack[1])
			buf, ok := m.Memory().Read(ptr, length)
			if !ok {
				return
			}
			e.alerts = append(e.alerts, string(buf))
		}), []api.ValueType{api.ValueTypeI32, api.ValueTypeI32}, nil).
		Export("host_alert").
		Instantiate(ctx)
	if err != nil {
		return fmt.Errorf("instantiate host module: %w", err)
	}

	mod, err := r.InstantiateWithConfig(ctx, data, wazero.NewModuleConfig().
		WithName("").
		WithStdout(nil).
		WithStderr(nil))
	if err != nil {
		return fmt.Errorf("instantiate wasm: %w", err)
	}

	e.r = r
	e.module = mod

	if err := e.initRule(ctx); err != nil {
		return err
	}

	return nil
}

func (e *Engine) initRule(ctx context.Context) error {
	nameBytes, err := e.callString(ctx, "rule_name")
	if err != nil {
		return fmt.Errorf("call rule_name: %w", err)
	}
	e.name = string(nameBytes)

	descBytes, err := e.callString(ctx, "rule_description")
	if err != nil {
		return fmt.Errorf("call rule_description: %w", err)
	}
	e.desc = string(descBytes)

	initFn := e.module.ExportedFunction("rule_init")
	if initFn == nil {
		return fmt.Errorf("rule_init not exported")
	}
	results, err := initFn.Call(ctx)
	if err != nil {
		return fmt.Errorf("rule_init: %w", err)
	}
	if len(results) > 0 && int32(results[0]) != 0 {
		return fmt.Errorf("rule_init failed with code %d", results[0])
	}
	return nil
}

func (e *Engine) callString(ctx context.Context, name string) ([]byte, error) {
	fn := e.module.ExportedFunction(name)
	if fn == nil {
		return nil, fmt.Errorf("%s not exported", name)
	}
	results, err := fn.Call(ctx)
	if err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("%s returned no value", name)
	}
	return e.readCString(uint32(results[0]))
}

func (e *Engine) readCString(ptr uint32) ([]byte, error) {
	mem := e.module.Memory()
	var buf []byte
	for {
		b, ok := mem.ReadByte(ptr)
		if !ok {
			return nil, fmt.Errorf("read cstring at %d", ptr)
		}
		if b == 0 {
			break
		}
		buf = append(buf, b)
		ptr++
	}
	return buf, nil
}

func (e *Engine) Name() string        { return e.name }
func (e *Engine) Description() string { return e.desc }
func (e *Engine) Alerts() []string    { return e.alerts }

func (e *Engine) Evaluate(ctx context.Context, jsonData []byte) (Result, error) {
	e.alerts = nil

	mem := e.module.Memory()
	prevPages := mem.Size() / 65536
	needed := (uint32(len(jsonData)) + 65535) / 65536
	if _, ok := mem.Grow(needed); !ok {
		return Pass, fmt.Errorf("grow memory for context data")
	}
	offset := prevPages * 65536

	if !mem.Write(offset, jsonData) {
		return Pass, fmt.Errorf("write context data")
	}

	evaluateFn := e.module.ExportedFunction("rule_evaluate")
	if evaluateFn == nil {
		return Pass, fmt.Errorf("rule_evaluate not exported")
	}

	results, err := evaluateFn.Call(ctx, uint64(offset), uint64(len(jsonData)))
	if err != nil {
		return Pass, fmt.Errorf("rule_evaluate: %w", err)
	}

	if len(results) == 0 {
		return Pass, nil
	}
	return Result(int32(results[0])), nil
}

func (e *Engine) Close(ctx context.Context) error {
	if e.r != nil {
		return e.r.Close(ctx)
	}
	return nil
}
