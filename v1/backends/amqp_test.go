package backends_test

import (
	"os"
	"testing"
	"time"

	"github.com/carousell/machinery/v1/backends"
	"github.com/carousell/machinery/v1/config"
	"github.com/carousell/machinery/v1/tasks"
	"github.com/stretchr/testify/assert"
)

var (
	amqpConfig *config.Config
)

func init() {
	amqpURL := os.Getenv("AMQP_URL")
	if amqpURL == "" {
		return
	}

	amqpConfig = &config.Config{
		Broker:        amqpURL,
		DefaultQueue:  "test_queue",
		ResultBackend: amqpURL,
		AMQP: &config.AMQPConfig{
			Exchange:      "test_exchange",
			ExchangeType:  "direct",
			BindingKey:    "test_task",
			PrefetchCount: 1,
		},
	}
}

func TestGroupCompletedAMQP(t *testing.T) {
	if os.Getenv("AMQP_URL") == "" {
		return
	}

	groupUUID := "testGroupUUID"
	groupTaskCount := 2
	task1 := &tasks.Signature{
		UUID:           "testTaskUUID1",
		GroupUUID:      groupUUID,
		GroupTaskCount: groupTaskCount,
	}
	task2 := &tasks.Signature{
		UUID:           "testTaskUUID2",
		GroupUUID:      groupUUID,
		GroupTaskCount: groupTaskCount,
	}

	backend := backends.NewAMQPBackend(amqpConfig)

	// Cleanup before the test
	backend.PurgeState(task1.UUID)
	backend.PurgeState(task2.UUID)
	backend.PurgeGroupMeta(groupUUID)

	groupCompleted, err := backend.GroupCompleted(groupUUID, groupTaskCount)
	if assert.NoError(t, err) {
		assert.False(t, groupCompleted)
	}

	backend.InitGroup(groupUUID, []string{task1.UUID, task2.UUID})

	groupCompleted, err = backend.GroupCompleted(groupUUID, groupTaskCount)
	if assert.NoError(t, err) {
		assert.False(t, groupCompleted)
	}

	backend.SetStatePending(task1)
	backend.SetStateStarted(task2)
	groupCompleted, err = backend.GroupCompleted(groupUUID, groupTaskCount)
	if assert.NoError(t, err) {
		assert.False(t, groupCompleted)
	}

	taskResults := []*tasks.TaskResult{new(tasks.TaskResult)}
	backend.SetStateSuccess(task1, taskResults)
	backend.SetStateSuccess(task2, taskResults)
	groupCompleted, err = backend.GroupCompleted(groupUUID, groupTaskCount)
	if assert.NoError(t, err) {
		assert.True(t, groupCompleted)
	}
}

func TestGetStateAMQP(t *testing.T) {
	if os.Getenv("AMQP_URL") == "" {
		return
	}

	signature := &tasks.Signature{
		UUID:      "testTaskUUID",
		GroupUUID: "testGroupUUID",
	}

	go func() {
		backend := backends.NewAMQPBackend(amqpConfig)
		backend.SetStatePending(signature)
		<-time.After(2 * time.Millisecond)
		backend.SetStateReceived(signature)
		<-time.After(2 * time.Millisecond)
		backend.SetStateStarted(signature)
		<-time.After(2 * time.Millisecond)

		taskResults := []*tasks.TaskResult{
			{
				Type:  "float64",
				Value: 2,
			},
		}
		backend.SetStateSuccess(signature, taskResults)
	}()

	backend := backends.NewAMQPBackend(amqpConfig)

	var (
		taskState *tasks.TaskState
		err       error
	)
	for {
		taskState, err = backend.GetState(signature.UUID)
		if taskState == nil {
			assert.Equal(t, "No state ready", err.Error())
			continue
		}

		assert.NoError(t, err)
		if taskState.IsCompleted() {
			break
		}
	}
}

func TestPurgeStateAMQP(t *testing.T) {
	if os.Getenv("AMQP_URL") == "" {
		return
	}

	signature := &tasks.Signature{
		UUID:      "testTaskUUID",
		GroupUUID: "testGroupUUID",
	}

	backend := backends.NewAMQPBackend(amqpConfig)

	backend.SetStatePending(signature)
	backend.SetStateReceived(signature)
	taskState, err := backend.GetState(signature.UUID)
	assert.NotNil(t, taskState)
	assert.NoError(t, err)

	backend.PurgeState(taskState.TaskUUID)
	taskState, err = backend.GetState(signature.UUID)
	assert.Nil(t, taskState)
	assert.Error(t, err)
}
