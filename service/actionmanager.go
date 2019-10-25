package service

import (
	"bytes"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"sort"
	"time"
)

type Action struct {
	Id          string
	StartedAt   *time.Time
	FinishedAt  *time.Time
	Command     string
	Description string
	Error       error
	log         *bytes.Buffer
}

func (this Action) Log() string {
	return this.log.String()
}
func (this Action) Finished() bool {
	return this.FinishedAt != nil
}
func (this Action) Status() string {
	if this.FinishedAt == nil {
		return "RUNNING"
	}
	if this.Error != nil {
		return "ERROR"
	}
	return "COMPLETED"
}

type ActionManager interface {
	LookupAction(id string) *Action
	RunningActions() []*Action
	CompletedActions() []*Action
	StartAction(command string) *Action
	LogAction(id string, log string) *Action
	Complete(id string) *Action
	CompleteWithMessage(id string, message string) *Action
	CompleteWithError(id string, err error) *Action
}

func CreateActionManager() ActionManager {
	return &actionManager{
		runningActions:   map[string]*Action{},
		completedActions: []*Action{},
	}
}

type actionManager struct {
	runningActions   map[string]*Action
	completedActions []*Action
}

func (this *actionManager) LookupAction(id string) *Action {
	action := this.runningActions[id]
	if action == nil {
		for _, a := range this.completedActions {
			if a.Id == id {
				return a
			}
		}
	}
	return action
}
func (this *actionManager) RunningActions() []*Action {
	actions := []*Action{}
	for _, v := range this.runningActions {
		actions = append(actions, v)
	}
	sort.Slice(actions, func(i, j int) bool {
		return actions[i].StartedAt.Nanosecond() < actions[j].StartedAt.Nanosecond()
	})
	return actions
}
func (this *actionManager) CompletedActions() []*Action {
	return this.completedActions
}
func (this *actionManager) StartAction(command string) *Action {
	var now = time.Now()
	var uuid, _ = uuid.NewUUID()
	a := Action{
		Id:        uuid.String(),
		StartedAt: &now,
		Command:   command,
		log:       &bytes.Buffer{},
	}
	this.runningActions[uuid.String()] = &a
	return &a
}
func (this *actionManager) LogAction(id string, log string) *Action {
	a := this.runningActions[id]
	if a != nil {
		if a.Finished() {
			logrus.Error("Cannot log action " + a.Command + ": already finished.")
		}
		a.log.WriteString(log)
	}
	return a
}
func (this *actionManager) Complete(id string) *Action {
	return this.CompleteWithMessage(id, "")
}
func (this *actionManager) CompleteWithMessage(id string, message string) *Action {
	now := time.Now()
	a := this.runningActions[id]
	if a != nil {
		if message != "" {
			a.log.WriteString(message + "\n")
		}
		delete(this.runningActions, id)
		a.FinishedAt = &now
		this.completedActions = append(this.completedActions, a)
	}
	return a
}
func (this *actionManager) CompleteWithError(id string, err error) *Action {
	now := time.Now()
	a := this.runningActions[id]
	if a != nil {
		a.log.WriteString(err.Error() + "\n")
		delete(this.runningActions, id)
		a.FinishedAt = &now
		a.Error = err
		this.completedActions = append(this.completedActions, a)
	}
	return a
}
