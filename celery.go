// Package celery provides a Golang client library for
// calling Celery tasks using an AMQP backend,
// it depends on channels created by http://github.com/streadway/amqp,
//
// for additional info see http://www.celeryproject.org
package celery

import (
	"encoding/json"
	"github.com/nu7hatch/gouuid"
	"github.com/streadway/amqp"
	"log"
	"time"
)

// Celery task representation,
// Task - task name,
// Id - task UUID,
// Args - optional task args,
// KWArgs - optional task kwargs,
// Retries - optional number of retries,
// ETA - optional time for a scheduled task,
// Expires - optional time for task expiration
type Task struct {
	Task    string
	Id      string
	Args    []string
	KWArgs  map[string]interface{}
	Retries int
	ETA     time.Time
	Expires time.Time
}

type FormattedTask struct {
	Task    string                 `json:"task"`
	Id      string                 `json:"id"`
	Args    []string               `json:"args,omitempty"`
	KWArgs  map[string]interface{} `json:"kwargs,omitempty"`
	Retries int                    `json:"retries,omitempty"`
	ETA     string                 `json:"eta,omitempty"`
	Expires string                 `json:"expires,omitempty"`
}

const timeFormat = "2006-01-02T15:04:05.999999"

// Returns a pointer to a new task object
func NewTask(task string, args []string, kwargs map[string]interface{}) (*Task, error) {
	id, err := uuid.NewV4()
	if err != nil {
		return nil, err
	}

	t := Task{
		Task:   task,
		Id:     id.String(),
		Args:   args,
		KWArgs: kwargs,
	}

	return &t, nil
}

// Marshals a Task object into JSON bytes array,
// time objects are converted to UTC and formatted in ISO8601
func (t *Task) MarshalJSON() ([]byte, error) {

	out := FormattedTask{
		Task:    t.Task,
		Id:      t.Id,
		Args:    t.Args,
		KWArgs:  t.KWArgs,
		Retries: t.Retries,
	}

	if !t.ETA.IsZero() {
		out.ETA = t.ETA.UTC().Format(timeFormat)
	}

	if !t.Expires.IsZero() {
		out.Expires = t.Expires.UTC().Format(timeFormat)
	}

	return json.Marshal(out)
}

func (t *Task) UnmarshalJSON(data []byte) error {
	task := FormattedTask{}
	err := json.Unmarshal(data, &task)

	t.Task = task.Task
	t.Id = task.Id
	t.Args = task.Args
	t.KWArgs = task.KWArgs
	t.Retries = task.Retries
	t.ETA, err = time.Parse(timeFormat, task.ETA)
	t.Expires, err = time.Parse(timeFormat, task.Expires)

	return err
}

// Publish a task to an AMQP channel,
// default exchange is "",
// default routing key is "celery"
func (t *Task) Publish(ch *amqp.Channel, exchange, key string) error {
	body, err := json.Marshal(t)
	if err != nil {
		return err
	}

	msg := amqp.Publishing{
		DeliveryMode:    amqp.Persistent,
		Timestamp:       time.Now(),
		ContentType:     "application/json",
		ContentEncoding: "utf-8",
		Body:            body,
	}

	return ch.Publish(exchange, key, false, false, msg)
}

func Consume(ch *amqp.Channel, queue, exchange, key string, messages chan<- Task) error {
	if err := ch.QueueBind(queue, key, exchange, false, nil); err != nil {
		log.Printf("Failed: %v", err)
		return err
	}

	deliveries, err := ch.Consume(queue, "", false, true, false, false, nil)
	if err != nil {
		log.Printf("Failed: %v", err)
		return err
	}

	for msg := range deliveries {
		task := &Task{}
		task.UnmarshalJSON(msg.Body)
		messages <- *task
		ch.Ack(msg.DeliveryTag, false)
	}

	return nil
}
