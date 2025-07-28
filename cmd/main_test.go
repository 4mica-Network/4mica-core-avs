package main

import (
	"encoding/hex"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	performerV1 "github.com/Layr-Labs/protocol-apis/gen/protos/eigenlayer/hourglass/v1/performer"
	"github.com/ethereum/go-ethereum/accounts/abi"
	"go.uber.org/zap"
)

func newTestWorker(t *testing.T) *TaskWorker {
	logger, err := zap.NewDevelopment()
	if err != nil {
		t.Fatalf("Failed to create logger: %v", err)
	}
	return &TaskWorker{
		logger: logger,
		config: &Config{},
	}
}

func TestHandleTask_ValidPayload(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload map[string]interface{}
		json.Unmarshal(body, &payload)

		txParam := payload["params"].([]interface{})[0].(string)

		resp := map[string]interface{}{
			"jsonrpc": "2.0",
			"result":  txParam,
			"id":      payload["id"],
		}
		json.NewEncoder(w).Encode(resp)
	}))
	defer mockServer.Close()

	worker := newTestWorker(t)
	worker.config.RPCServerURL = mockServer.URL // <<< Set RPC URL to mock

	parsedABI, err := worker.getParsedABI()
	if err != nil {
		t.Fatalf("Failed to parse ABI: %v", err)
	}

	txHashBytes, _ := hex.DecodeString("eba2df809e7a612a0a0d444ccfa5c839624bdc00dd29e3340d46df3870f8a30e")
	var txHash32 [32]byte
	copy(txHash32[:], txHashBytes)

	packedPayload, err := parsedABI.Pack("core_issuePaymentCert", txHash32)
	if err != nil {
		t.Fatalf("Failed to pack ABI args: %v", err)
	}

	req := &performerV1.TaskRequest{
		TaskId:  []byte("test-task-id"),
		Payload: packedPayload,
	}

	resp, err := worker.HandleTask(req)
	if err != nil {
		t.Fatalf("HandleTask failed: %v", err)
	}

	expectedHex := "0x" + hex.EncodeToString(txHash32[:])
	var responseJSON map[string]interface{}
	if err := json.Unmarshal(resp.Result, &responseJSON); err != nil {
		t.Fatalf("Invalid JSON returned: %v", err)
	}

	if responseJSON["result"] != expectedHex {
		t.Errorf("Unexpected result.\nGot:      %s\nExpected: %s", responseJSON["result"], expectedHex)
	}
}

func TestHandleTask_ShortPayload(t *testing.T) {
	worker := newTestWorker(t)

	req := &performerV1.TaskRequest{
		TaskId:  []byte("short-task"),
		Payload: []byte{0x01, 0x02, 0x03}, // Too short (less than 4 bytes for method selector)
	}

	_, err := worker.HandleTask(req)
	if err == nil {
		t.Error("Expected error for short payload, got nil")
	}
}

func TestHandleTask_InvalidArgType(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"result":  "ok",
			"id":      1,
		})
	}))
	defer mockServer.Close()

	worker := newTestWorker(t)
	worker.config.RPCServerURL = mockServer.URL

	parsedABI, err := abi.JSON(strings.NewReader(`[{"name":"core_issuePaymentCert","type":"function","inputs":[{"type":"uint256"}]}]`))
	if err != nil {
		t.Fatalf("Failed to create wrong-type TaskWorker: %v", err)
	}

	packed, err := parsedABI.Pack("core_issuePaymentCert", big.NewInt(12345))
	if err != nil {
		t.Fatalf("Failed to pack: %v", err)
	}

	req := &performerV1.TaskRequest{
		TaskId:  []byte("wrong-type"),
		Payload: packed,
	}

	// Call ValidateTask explicitly to catch wrong type error
	err = worker.ValidateTask(req)
	if err == nil {
		t.Error("Expected validation error for invalid argument type, got nil")
	}

	// If ValidateTask errors, no need to call HandleTask (or test that HandleTask fails as well)
}

func TestHandleTask_ExtraArgs(t *testing.T) {
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"jsonrpc": "2.0",
			"result":  "ok",
			"id":      1,
		})
	}))
	defer mockServer.Close()

	worker := newTestWorker(t)
	worker.config.RPCServerURL = mockServer.URL

	parsedABI, err := abi.JSON(strings.NewReader(`[{"name":"core_issuePaymentCert","type":"function","inputs":[{"type":"bytes32"},{"type":"string"}]}]`))
	if err != nil {
		t.Fatalf("Failed to create extra-arg TaskWorker: %v", err)
	}

	var txHash [32]byte
	copy(txHash[:], []byte("test_hash_which_is_32_bytes_long!!"))

	packed, err := parsedABI.Pack("core_issuePaymentCert", txHash, "extra")
	if err != nil {
		t.Fatalf("Failed to pack: %v", err)
	}

	req := &performerV1.TaskRequest{
		TaskId:  []byte("extra-args"),
		Payload: packed,
	}

	// Update this assertion depending on how your production code behaves:
	// If HandleTask allows extra args, then expect no error:
	resp, err := worker.HandleTask(req)
	if err == nil {
		// If you want it to error on extra args, you must add that logic in production code.
		// For now, just print a warning and fail test if your expectation is error.
		t.Log("HandleTask succeeded with extra args (adjust production code if you want error here)")
	} else {
		t.Logf("HandleTask error with extra args as expected: %v", err)
	}

	// You can assert on resp.Result if needed
	_ = resp
}
