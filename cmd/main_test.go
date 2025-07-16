package main

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"

	performerV1 "github.com/Layr-Labs/protocol-apis/gen/protos/eigenlayer/hourglass/v1/performer"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"github.com/ethereum/go-ethereum/common"
	"go.uber.org/zap"
)

func Test_TaskRequestPayload(t *testing.T) {
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	defer logger.Sync()

	taskWorker := NewTaskWorker(logger)

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

	payloadHex := hex.EncodeToString(packed)

	taskRequest := &performerV1.TaskRequest{
		TaskId:   []byte("test-task-id"),
		Payload:  []byte(payloadHex),
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

	// -------------------------------
	// Compute expected SHA256 result
	// -------------------------------
	expectedHash := sha256.Sum256(txHash.Bytes())
	expectedHashHex := hex.EncodeToString(expectedHash[:])

	// -------------------------------
	// Compare with actual result
	// -------------------------------
	if string(resp.Result) != expectedHashHex {
		t.Errorf("Unexpected result.\nGot:      %s\nExpected: %s", string(resp.Result), expectedHashHex)
	} else {
		t.Logf("Success: SHA256(txHash) = %s", string(resp.Result))
	}
}
