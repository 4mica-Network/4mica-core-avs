package main

import (
	"crypto/sha256"
	"encoding/hex"
	"math/big"
	"strings"
	"testing"

	performerV1 "github.com/Layr-Labs/protocol-apis/gen/protos/eigenlayer/hourglass/v1/performer"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"go.uber.org/zap"
)

// Shared ABI JSON and method name for tests
const (
	testABIJSON = `[{"name":"dummy","type":"function","inputs":[{"type":"bytes32"}]}]`
	testMethod  = "dummy"
)

// Setup shared logger and worker for tests
func newTestWorker(t *testing.T) *TaskWorker {
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}

	worker, err := NewTaskWorker(logger, TaskWorkerConfig{
		ABIJSON: testABIJSON,
		Method:  testMethod,
	})
	if err != nil {
		t.Fatalf("Failed to create TaskWorker: %v", err)
	}
	return worker
}

func TestValidateTask_ValidPayload(t *testing.T) {
	taskWorker := newTestWorker(t)

	parsedABI, err := abi.JSON(strings.NewReader(testABIJSON))
	if err != nil {
		t.Fatalf("Failed to parse ABI: %v", err)
	}

	txHash := common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	packed, err := parsedABI.Pack(testMethod, txHash)
	if err != nil {
		t.Fatalf("Failed to pack ABI args: %v", err)
	}

	taskRequest := &performerV1.TaskRequest{
		TaskId:   []byte("test-task-id"),
		Payload:  packed,
		Metadata: []byte("test-metadata"),
	}

	err = taskWorker.ValidateTask(taskRequest)
	if err != nil {
		t.Errorf("ValidateTask failed: %v", err)
	}

	resp, err := taskWorker.HandleTask(taskRequest)
	if err != nil {
		t.Errorf("HandleTask failed: %v", err)
	}

	expectedHash := sha256.Sum256(taskRequest.Payload)
	expectedHashHex := hex.EncodeToString(expectedHash[:])
	if string(resp.Result) != expectedHashHex {
		t.Errorf("Unexpected result.\nGot:      %s\nExpected: %s", string(resp.Result), expectedHashHex)
	}
}

func TestValidateTask_ShortPayload(t *testing.T) {
	taskWorker := newTestWorker(t)

	taskRequest := &performerV1.TaskRequest{
		TaskId:   []byte("short-task"),
		Payload:  []byte{0x01, 0x02, 0x03},
		Metadata: nil,
	}

	err := taskWorker.ValidateTask(taskRequest)
	if err == nil {
		t.Error("Expected validation error for short payload, got nil")
	}
}

func TestValidateTask_WrongInputType(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	wrongWorker, err := NewTaskWorker(logger, TaskWorkerConfig{
		ABIJSON: `[{"name":"dummy","type":"function","inputs":[{"type":"uint256"}]}]`,
		Method:  "dummy",
	})
	if err != nil {
		t.Fatalf("Failed to create wrong-type TaskWorker: %v", err)
	}

	wrongABI, _ := abi.JSON(strings.NewReader(`[{"name":"dummy","type":"function","inputs":[{"type":"uint256"}]}]`))
	bigIntValue := big.NewInt(123)
	packed, err := wrongABI.Pack("dummy", bigIntValue)
	if err != nil {
		t.Fatalf("Failed to pack wrong input: %v", err)
	}

	taskRequest := &performerV1.TaskRequest{
		TaskId:   []byte("wrong-type-task"),
		Payload:  packed,
		Metadata: nil,
	}

	err = wrongWorker.ValidateTask(taskRequest)
	if err == nil {
		t.Error("Expected validation error for wrong input type, got nil")
	}
}

func TestValidateTask_ExtraArguments(t *testing.T) {
	logger, _ := zap.NewDevelopment()
	worker, err := NewTaskWorker(logger, TaskWorkerConfig{
		ABIJSON: `[{"name":"dummy","type":"function","inputs":[{"type":"bytes32"},{"type":"bytes32"}]}]`,
		Method:  "dummy",
	})
	if err != nil {
		t.Fatalf("Failed to create extra-arg TaskWorker: %v", err)
	}

	multiABI, _ := abi.JSON(strings.NewReader(`[{"name":"dummy","type":"function","inputs":[{"type":"bytes32"},{"type":"bytes32"}]}]`))
	txHash1 := common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	txHash2 := common.HexToHash("0xabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcdefabcd")

	packed, err := multiABI.Pack("dummy", txHash1, txHash2)
	if err != nil {
		t.Fatalf("Failed to pack extra args: %v", err)
	}

	taskRequest := &performerV1.TaskRequest{
		TaskId:   []byte("extra-arg-task"),
		Payload:  packed,
		Metadata: nil,
	}

	err = worker.ValidateTask(taskRequest)
	if err == nil {
		t.Error("Expected validation error for extra argument, got nil")
	}
}
