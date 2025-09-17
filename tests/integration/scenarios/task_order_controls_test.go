package scenarios

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"prismtest/internal/assertx"
)

func TestTaskOrderControls(t *testing.T) {
	client := newPrismApiClient(t)
	prefix := fmt.Sprintf("order-%d", time.Now().UnixNano())

	create := func(title, category string, idx int) {
		payload := map[string]any{"title": title, "category": category}
		key := fmt.Sprintf("ik-%s-create-%d", prefix, idx)
		if resp, err := client.PostJSON("/api/commands", []command{{
			IdempotencyKey: key,
			EntityType:     "task",
			Type:           "create-task",
			Data:           payload,
		}}, nil); err != nil || resp.StatusCode >= 300 {
			t.Fatalf("create %s: status %d err %v", title, resp.StatusCode, err)
		}
	}

	// create three tasks in the normal lane
	normalTitles := make([]string, 3)
	for i := 0; i < len(normalTitles); i++ {
		normalTitles[i] = fmt.Sprintf("%s-normal-%d", prefix, i)
		create(normalTitles[i], "normal", i)
	}

	// create two tasks in the fun lane to establish an existing order baseline
	funTitles := make([]string, 2)
	for i := 0; i < len(funTitles); i++ {
		funTitles[i] = fmt.Sprintf("%s-fun-%d", prefix, i)
		create(funTitles[i], "fun", i+len(normalTitles))
	}

	var normalIDs [3]string

	pollTasks(t, client, "all seeded tasks to exist", func(ts []task) bool {
		foundNormal := 0
		foundFun := 0
		for _, tk := range ts {
			if strings.HasPrefix(tk.Title, prefix+"-normal-") {
				for idx, title := range normalTitles {
					if tk.Title == title {
						normalIDs[idx] = tk.ID
						break
					}
				}
				foundNormal++
			}
			if strings.HasPrefix(tk.Title, prefix+"-fun-") {
				foundFun++
			}
		}
		return foundNormal == len(normalTitles) && foundFun == len(funTitles)
	})

	for idx, id := range normalIDs {
		if id == "" {
			t.Fatalf("missing normal task id at index %d", idx)
		}
	}

	// swap the first two normal tasks to emulate the arrow controls
	reorderKey := fmt.Sprintf("ik-%s-reorder", prefix)
	commands := []command{
		{
			IdempotencyKey: reorderKey + "-up",
			EntityType:     "task",
			Type:           "update-task",
			Data: map[string]any{
				"id":    normalIDs[0],
				"order": 1,
			},
		},
		{
			IdempotencyKey: reorderKey + "-down",
			EntityType:     "task",
			Type:           "update-task",
			Data: map[string]any{
				"id":    normalIDs[1],
				"order": 0,
			},
		},
	}
	if resp, err := client.PostJSON("/api/commands", commands, nil); err != nil || resp.StatusCode >= 300 {
		t.Fatalf("reorder commands: status %d err %v", resp.StatusCode, err)
	}

	pollTasks(t, client, "normal tasks swapped", func(ts []task) bool {
		for _, tk := range ts {
			if tk.ID == normalIDs[0] && tk.Order != 1 {
				return false
			}
			if tk.ID == normalIDs[1] && tk.Order != 0 {
				return false
			}
		}
		return true
	})

	// move the last normal task to the fun lane; it should be appended to the end
	moveKey := fmt.Sprintf("ik-%s-move", prefix)
	lastNormalID := normalIDs[2]
	targetOrder := len(funTitles)
	if resp, err := client.PostJSON("/api/commands", []command{{
		IdempotencyKey: moveKey,
		EntityType:     "task",
		Type:           "update-task",
		Data: map[string]any{
			"id":       lastNormalID,
			"category": "fun",
			"order":    targetOrder,
		},
	}}, nil); err != nil || resp.StatusCode >= 300 {
		t.Fatalf("move command: status %d err %v", resp.StatusCode, err)
	}

	tasks := pollTasks(t, client, "task appended to fun lane", func(ts []task) bool {
		moved := false
		for _, tk := range ts {
			if tk.ID == lastNormalID {
				moved = tk.Category == "fun" && tk.Order == targetOrder
				break
			}
		}
		return moved
	})

	// verify the fun lane ordering is contiguous and the moved task sits at the end
	var funOrders []int
	for _, tk := range tasks {
		if tk.Category == "fun" && strings.HasPrefix(tk.Title, prefix) {
			funOrders = append(funOrders, tk.Order)
		}
	}
	assertx.Equal(t, len(funTitles)+1, len(funOrders))
	found := false
	for _, ord := range funOrders {
		if ord == targetOrder {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected to find order %d in %v", targetOrder, funOrders)
	}
}
