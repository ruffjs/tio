package job

import (
	"container/heap"
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

var tests = []struct {
	name  string
	tasks []*Task
	want  []*Task
}{
	{"order",
		[]*Task{
			{CreatedAt: 5, Status: TaskQueued},
			{CreatedAt: 2, Status: TaskInProgress},
			{CreatedAt: 6, Status: TaskQueued},
			{CreatedAt: 4, Status: TaskSent},
			{CreatedAt: 22, Status: TaskInProgress},
			{CreatedAt: 1, Status: TaskQueued},
		},
		[]*Task{
			{CreatedAt: 2, Status: TaskInProgress},
			{CreatedAt: 22, Status: TaskInProgress},
			{CreatedAt: 4, Status: TaskSent},
			{CreatedAt: 1, Status: TaskQueued},
			{CreatedAt: 5, Status: TaskQueued},
			{CreatedAt: 6, Status: TaskQueued},
		},
	},
}

func TestPriorityQueue(t *testing.T) {
	for _, tt := range tests {
		st := tt
		t.Run(st.name, func(t *testing.T) {
			q := make(taskPriorityQueue, 0)
			for _, task := range st.tasks {
				item := TaskItem{Task: task}
				heap.Push(&q, &item)
			}
			for i, ot := range st.want {
				f := heap.Pop(&q).(*TaskItem)
				fmt.Printf("%d , %s %d \n", i, f.Task.Status, f.Task.CreatedAt)

				require.Equal(t, ot, f.Task, "pop")
			}
		})
	}
}

func TestTaskQueue(t *testing.T) {
	for _, tt := range tests {
		st := tt
		t.Run(st.name, func(t *testing.T) {
			q := NewTaskQueue()

			for _, task := range st.tasks {
				q.Push(task)
			}
			for i, ot := range st.want {
				pk := q.Peek()
				require.Equal(t, ot, pk, "peek")

				f := q.Pop()
				fmt.Printf("%d , %s %d \n", i, f.Status, f.CreatedAt)
				require.Equal(t, ot, f, "pop")
			}
		})
	}
}
