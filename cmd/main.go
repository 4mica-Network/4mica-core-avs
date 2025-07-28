package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Layr-Labs/hourglass-monorepo/ponos/pkg/performer/server"
	performerV1 "github.com/Layr-Labs/protocol-apis/gen/protos/eigenlayer/hourglass/v1/performer"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"go.uber.org/zap"
)

type Config struct {
	RPCServerURL string
}

type TaskWorker struct {
	logger *zap.Logger
	config *Config
}

func NewTaskWorker(logger *zap.Logger, config *Config) *TaskWorker {
	return &TaskWorker{
		logger: logger,
		config: config,
	}
}

func (tw *TaskWorker) ValidateTask(t *performerV1.TaskRequest) error {
	logger := tw.logger.Sugar()
	tw.logger.Sugar().Infow("Validating task", zap.Any("task", t))
	const (
		jsonRPCVersion       = "2.0"
		jsonContentType      = "application/json"
		jsonRPCMethodName    = "core_issuePaymentCert"
		expectedArgCount     = 1
		expectedArgTypeSize  = 32
		methodSelectorLength = 4
	)

	parsedABI, err := tw.getParsedABI()
	if err != nil {
		logger.Errorw("Failed to parse ABI", "error", err)
		return fmt.Errorf("failed to parse ABI: %w", err)
	}

	method, ok := parsedABI.Methods["dummy"]
	if !ok {
		return fmt.Errorf("ABI method 'dummy' not found")
	}

	if len(t.Payload) < methodSelectorLength {
		logger.Errorw("Payload too short", "min_required", methodSelectorLength, "actual", len(t.Payload))
		return fmt.Errorf("payload too short: expected at least %d bytes", methodSelectorLength)
	}

	args, err := method.Inputs.Unpack(t.Payload[methodSelectorLength:])
	if err != nil {
		logger.Errorw("Failed to unpack method arguments", "error", err)
		return fmt.Errorf("failed to unpack method arguments: %w", err)
	}

	if len(args) != expectedArgCount {
		logger.Errorw("Unexpected number of arguments", "expected", expectedArgCount, "actual", len(args))
		return fmt.Errorf("unexpected number of arguments: expected %d, got %d", expectedArgCount, len(args))
	}

	return nil
}

func (tw *TaskWorker) HandleTask(t *performerV1.TaskRequest) (*performerV1.TaskResponse, error) {
	const (
		jsonRPCVersion       = "2.0"
		jsonContentType      = "application/json"
		jsonRPCMethodName    = "core_issuePaymentCert"
		expectedArgCount     = 1
		expectedArgTypeSize  = 32
		methodSelectorLength = 4
	)

	logger := tw.logger.Sugar()
	logger.Infow("Handling task", "task_id", t.TaskId, "payload_length", len(t.Payload))

	parsedABI, err := tw.getParsedABI()
	if err != nil {
		logger.Errorw("Failed to parse ABI", "error", err)
		return nil, fmt.Errorf("failed to parse ABI: %w", err)
	}

	method, ok := parsedABI.Methods[jsonRPCMethodName]
	if !ok {
		logger.Errorw("ABI method not found", "method", jsonRPCMethodName)
		return nil, fmt.Errorf("ABI method '%s' not found", jsonRPCMethodName)
	}

	if len(t.Payload) < methodSelectorLength {
		logger.Errorw("Payload too short", "min_required", methodSelectorLength, "actual", len(t.Payload))
		return nil, fmt.Errorf("payload too short: expected at least %d bytes", methodSelectorLength)
	}

	args, err := method.Inputs.Unpack(t.Payload[methodSelectorLength:])
	if err != nil {
		logger.Errorw("Failed to unpack method arguments", "error", err)
		return nil, fmt.Errorf("failed to unpack method arguments: %w", err)
	}

	if len(args) != expectedArgCount {
		logger.Errorw("Unexpected number of arguments", "expected", expectedArgCount, "actual", len(args))
		return nil, fmt.Errorf("unexpected number of arguments: expected %d, got %d", expectedArgCount, len(args))
	}

	txHashArg := args[0]
	txHash, valid := txHashArg.([expectedArgTypeSize]byte)
	if !valid {
		logger.Errorw("Invalid argument type for txHash", "expected", fmt.Sprintf("[%d]byte", expectedArgTypeSize), "actual", fmt.Sprintf("%T", txHashArg))
		return nil, fmt.Errorf("unexpected argument type: expected [%d]byte, got %T", expectedArgTypeSize, txHashArg)
	}

	jsonPayload := map[string]interface{}{
		"jsonrpc": jsonRPCVersion,
		"method":  jsonRPCMethodName,
		"params":  []interface{}{fmt.Sprintf("0x%x", txHash)},
		"id":      1,
	}

	jsonData, err := json.Marshal(jsonPayload)
	if err != nil {
		logger.Errorw("Failed to marshal JSON payload", "error", err)
		return nil, fmt.Errorf("failed to marshal JSON-RPC payload: %w", err)
	}

	resp, err := http.Post(tw.config.RPCServerURL, jsonContentType, bytes.NewBuffer(jsonData))
	if err != nil {
		logger.Errorw("HTTP request failed", "url", tw.config.RPCServerURL, "error", err)
		return nil, fmt.Errorf("failed to send POST request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Errorw("Failed to read response body", "error", err)
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	logger.Infow("Received response from RPC server", "status_code", resp.StatusCode, "body", string(body))

	return &performerV1.TaskResponse{
		TaskId: t.TaskId,
		Result: body,
	}, nil
}

// getParsedABI returns the parsed ABI for the contract. Consider caching this at init.
func (tw *TaskWorker) getParsedABI() (abi.ABI, error) {
	const contractABI = `
	[{
		"name": "core_issuePaymentCert",
		"type": "function",
		"inputs": [{"name": "txHash", "type": "bytes32"}]
	}]
	`
	return abi.JSON(strings.NewReader(contractABI))
}

func main() {
	ctx := context.Background()
	l, _ := zap.NewProduction()

	config := &Config{
		RPCServerURL: "http://localhost:3000",
	}

	w := NewTaskWorker(l, config)

	pp, err := server.NewPonosPerformerWithRpcServer(&server.PonosPerformerConfig{
		Port:    8080,
		Timeout: 5 * time.Second,
	}, w, l)
	if err != nil {
		logger.Fatal("failed to create performer", zap.Error(err))
	}

	if err := ponos.Start(ctx); err != nil {
		logger.Fatal("failed to start performer", zap.Error(err))
	}
}
