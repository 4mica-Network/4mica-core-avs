package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/Layr-Labs/hourglass-monorepo/ponos/pkg/performer/server"
	performerV1 "github.com/Layr-Labs/protocol-apis/gen/protos/eigenlayer/hourglass/v1/performer"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"go.uber.org/zap"
)

const (
	defaultPort    = 8080
	defaultTimeout = 5 * time.Second

	dummyABIJSON = `[{"name":"dummy","type":"function","inputs":[{"name":"txHash","type":"bytes32"}]}]`
)

type TaskWorkerConfig struct {
	ABIJSON string
	Method  string
}

type TaskWorker struct {
	logger *zap.Logger
	abi    abi.ABI
	method abi.Method
}

func NewTaskWorker(logger *zap.Logger, cfg TaskWorkerConfig) (*TaskWorker, error) {
	parsedABI, err := abi.JSON(strings.NewReader(cfg.ABIJSON))
	if err != nil {
		return nil, fmt.Errorf("failed to parse ABI: %w", err)
	}

	method, ok := parsedABI.Methods[cfg.Method]
	if !ok {
		return nil, fmt.Errorf("ABI method %q not found", cfg.Method)
	}

	return &TaskWorker{
		logger: logger,
		abi:    parsedABI,
		method: method,
	}, nil
}

func (tw *TaskWorker) ValidateTask(t *performerV1.TaskRequest) error {
	tw.logger.Sugar().Infow("Validating task", zap.ByteString("task_id", t.TaskId))

	if len(t.Payload) < 4 {
		return fmt.Errorf("payload too short to contain method selector")
	}

	if !equalBytes(t.Payload[:4], tw.method.ID) {
		return fmt.Errorf("invalid method selector")
	}

	encodedArgs := t.Payload[4:]

	expectedArgEncoding, _ := tw.method.Inputs.Pack([32]byte{})
	if len(encodedArgs) != len(expectedArgEncoding) {
		return fmt.Errorf("unexpected argument length: got %d, want %d", len(encodedArgs), len(expectedArgEncoding))
	}

	args, err := tw.method.Inputs.Unpack(encodedArgs)
	if err != nil {
		return fmt.Errorf("failed to unpack arguments: %w", err)
	}

	if len(args) != 1 {
		return fmt.Errorf("expected 1 argument, got %d", len(args))
	}

	if _, ok := args[0].([32]byte); !ok {
		return fmt.Errorf("expected argument type [32]byte, got %T", args[0])
	}

	return nil
}

func (tw *TaskWorker) HandleTask(t *performerV1.TaskRequest) (*performerV1.TaskResponse, error) {
	tw.logger.Sugar().Infow("Handling task",
		zap.ByteString("task_id", t.TaskId),
		zap.Binary("payload", t.Payload),
	)

	if len(t.Payload) < 4 {
		tw.logger.Sugar().Error("payload too short")
		return nil, fmt.Errorf("payload too short")
	}

	args, err := tw.method.Inputs.Unpack(t.Payload[4:])
	if err != nil {
		tw.logger.Sugar().Errorf("failed to unpack: %v", err)
		return nil, fmt.Errorf("failed to unpack: %w", err)
	}

	if len(args) != 1 {
		tw.logger.Sugar().Errorf("unexpected number of arguments: %d", len(args))
		return nil, fmt.Errorf("unexpected number of arguments: %d", len(args))
	}

	txHash, ok := args[0].([32]byte)
	if !ok {
		tw.logger.Sugar().Errorf("unexpected argument type: %T", args[0])
		return nil, fmt.Errorf("unexpected argument type: %T", args[0])
	}

	tw.logger.Sugar().Infof("Extracted txHash: 0x%x", txHash)

	hash := sha256.Sum256(t.Payload)
	resultStr := hex.EncodeToString(hash[:])

	tw.logger.Sugar().Infow("Response to task",
		zap.ByteString("task_id", t.TaskId),
		zap.String("response", resultStr),
	)

	return &performerV1.TaskResponse{
		TaskId: t.TaskId,
		Result: []byte(resultStr),
	}, nil
}

func equalBytes(a, b []byte) bool {
	return len(a) == len(b) && string(a) == string(b)
}

func main() {
	ctx := context.Background()

	logger, err := zap.NewProduction()
	if err != nil {
		panic(fmt.Errorf("failed to initialize logger: %w", err))
	}
	defer logger.Sync()

	workerCfg := TaskWorkerConfig{
		ABIJSON: dummyABIJSON,
		Method:  "dummy",
	}

	worker, err := NewTaskWorker(logger, workerCfg)
	if err != nil {
		logger.Fatal("failed to initialize TaskWorker", zap.Error(err))
	}

	ponos, err := server.NewPonosPerformerWithRpcServer(&server.PonosPerformerConfig{
		Port:    defaultPort,
		Timeout: defaultTimeout,
	}, worker, logger)
	if err != nil {
		logger.Fatal("failed to create performer", zap.Error(err))
	}

	if err := ponos.Start(ctx); err != nil {
		logger.Fatal("failed to start performer", zap.Error(err))
	}
}
