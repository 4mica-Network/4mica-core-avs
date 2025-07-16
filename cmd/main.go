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

	// ------------------------------------------------------------------------
	// Implement your AVS task validation logic here
	// ------------------------------------------------------------------------
	// This is where the Perfomer will validate the task request data.
	// E.g. the Perfomer may validate that the request params are well formed and adhere to a schema.

	return nil
}

func (tw *TaskWorker) HandleTask(t *performerV1.TaskRequest) (*performerV1.TaskResponse, error) {
	tw.logger.Sugar().Infow("Handling task",
		zap.Any("task_id", t.TaskId),
		zap.String("payload", string(t.Payload)),
	)

	// ABI definition
	const abiJSON = `[{"name":"dummy","type":"function","inputs":[{"type":"bytes32"}]}]`
	parsedABI, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		tw.logger.Sugar().Errorf("failed to parse ABI: %v", err)
		return nil, fmt.Errorf("failed to parse ABI: %w", err)
	}

	// Decode hex-encoded payload string
	payloadStr := string(t.Payload)
	payload, err := hex.DecodeString(payloadStr)
	if err != nil {
		tw.logger.Sugar().Errorf("invalid hex string: %v", err)
		return nil, fmt.Errorf("invalid hex string: %w", err)
	}

	// Validate method existence
	method, ok := parsedABI.Methods["dummy"]
	if !ok {
		tw.logger.Sugar().Error("ABI method 'dummy' not found")
		return nil, fmt.Errorf("ABI method 'dummy' not found")
	}

	// Unpack the data
	args, err := method.Inputs.Unpack(payload[4:]) // skip function selector (first 4 bytes)
	if err != nil {
		tw.logger.Sugar().Errorf("failed to unpack: %v", err)
		return nil, fmt.Errorf("failed to unpack: %w", err)
	}

	// Extract the bytes32 argument (tx hash)
	if len(args) != 1 {
		tw.logger.Sugar().Errorf("unexpected number of arguments: %d", len(args))
		return nil, fmt.Errorf("unexpected number of arguments: %d", len(args))
	}

	txHash, ok := args[0].([32]byte)
	if !ok {
		tw.logger.Sugar().Errorf("unexpected argument type: %T", args[0])
		return nil, fmt.Errorf("unexpected argument type: %T", args[0])
	}

	// For now, we just log and echo back the hash string
	// Hash the txHash with SHA256
	hash := sha256.Sum256(txHash[:])
	resultStr := hex.EncodeToString(hash[:])
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
