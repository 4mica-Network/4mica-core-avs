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

// Setup shared logger and worker for tests
func newTestWorker(t *testing.T) *TaskWorker {
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	return NewTaskWorker(logger)
}

func TestValidateTask_ValidPayload(t *testing.T) {
	taskWorker := newTestWorker(t)

	const abiJSON = `[{"name":"dummy","type":"function","inputs":[{"type":"bytes32"}]}]`
	parsedABI, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		t.Fatalf("Failed to parse ABI: %v", err)
	}

	txHash := common.HexToHash("0x1234567890abcdef1234567890abcdef1234567890abcdef1234567890abcdef")
	packed, err := parsedABI.Pack("dummy", txHash)
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
	taskWorker := newTestWorker(t)

	const abiJSON = `[{"name":"dummy","type":"function","inputs":[{"type":"uint256"}]}]`
	wrongABI, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		t.Fatalf("Failed to parse ABI: %v", err)
	}

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

	err = taskWorker.ValidateTask(taskRequest)
	if err == nil {
		t.Error("Expected validation error for wrong input type, got nil")
	}
}
func TestValidateTask_ExtraArguments(t *testing.T) {
	taskWorker := newTestWorker(t)

	const abiJSON = `[{"name":"dummy","type":"function","inputs":[{"type":"bytes32"},{"type":"bytes32"}]}]`
	multiABI, err := abi.JSON(strings.NewReader(abiJSON))
	if err != nil {
		t.Fatalf("Failed to parse ABI: %v", err)
	}

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

	err = taskWorker.ValidateTask(taskRequest)
	if err == nil {
		t.Error("Expected validation error for extra argument, got nil")
	}
}
