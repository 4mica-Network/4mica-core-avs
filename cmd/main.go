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

// This offchain binary is run by Operators running the Hourglass Executor. It contains
// the business logic of the AVS and performs worked based on the tasked sent to it.
// The Hourglass Aggregator ingests tasks from the TaskMailbox and distributes work
// to Executors configured to run the AVS Performer. Performers execute the work and
// return the result to the Executor where the result is signed and return to the
// Aggregator to place in the outbox once the signing threshold is met.

type TaskWorker struct {
	logger *zap.Logger
}

func NewTaskWorker(logger *zap.Logger) *TaskWorker {
	return &TaskWorker{
		logger: logger,
	}
}

func (tw *TaskWorker) ValidateTask(t *performerV1.TaskRequest) error {
	tw.logger.Sugar().Infow("Validating task",
		zap.Any("task", t),
	)

	const abiJSON = `[{"name":"dummy","type":"function","inputs":[{"name":"txHash","type":"bytes32"}]}]`
	parsedABI, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		return fmt.Errorf("failed to parse ABI: %w", err)
	}

	method, ok := parsedABI.Methods["dummy"]
	if !ok {
		return fmt.Errorf("ABI method 'dummy' not found")
	}

	expectedSelector := method.ID
	if len(t.Payload) < 4 || !strings.HasPrefix(hex.EncodeToString(t.Payload[:4]), hex.EncodeToString(expectedSelector)) {
		return fmt.Errorf("invalid method selector")
	}

	encodedArgs := t.Payload[4:]

	// Validate length
	dummyInput := [32]byte{}
	expectedArgEncoding, _ := method.Inputs.Pack(dummyInput)
	if len(encodedArgs) != len(expectedArgEncoding) {
		return fmt.Errorf("unexpected argument length: got %d, want %d", len(encodedArgs), len(expectedArgEncoding))
	}

	args, err := method.Inputs.Unpack(encodedArgs)
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
		zap.Any("task_id", t.TaskId),
		zap.Binary("payload", t.Payload),
	)

	// ABI definition: dummy(bytes32)
	const abiJSON = `[{"name":"dummy","type":"function","inputs":[{"name":"txHash","type":"bytes32"}]}]`
	parsedABI, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		tw.logger.Sugar().Errorf("failed to parse ABI: %v", err)
		return nil, fmt.Errorf("failed to parse ABI: %w", err)
	}

	// Validate method existence
	method, ok := parsedABI.Methods["dummy"]
	if !ok {
		tw.logger.Sugar().Error("ABI method 'dummy' not found")
		return nil, fmt.Errorf("ABI method 'dummy' not found")
	}

	// Ensure payload has at least 4 bytes for function selector
	if len(t.Payload) < 4 {
		tw.logger.Sugar().Error("payload too short")
		return nil, fmt.Errorf("payload too short")
	}

	// Unpack arguments from payload, skipping first 4 bytes (function selector)
	args, err := method.Inputs.Unpack(t.Payload[4:])
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

	// Generate SHA-256 hash of the original payload
	hash := sha256.Sum256(t.Payload)
	resultStr := hex.EncodeToString(hash[:])

	tw.logger.Sugar().Infow("Response to task",
		zap.Any("task_id", t.TaskId),
		zap.String("response", resultStr),
	)

	return &performerV1.TaskResponse{
		TaskId: t.TaskId,
		Result: []byte(resultStr),
	}, nil
}

func main() {
	ctx := context.Background()
	l, _ := zap.NewProduction()

	w := NewTaskWorker(l)

	pp, err := server.NewPonosPerformerWithRpcServer(&server.PonosPerformerConfig{
		Port:    8080,
		Timeout: 5 * time.Second,
	}, w, l)
	if err != nil {
		panic(fmt.Errorf("failed to create performer: %w", err))
	}

	if err := pp.Start(ctx); err != nil {
		panic(err)
	}
}
