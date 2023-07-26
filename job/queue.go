package job

import "container/heap"

var taskStatusPriority = map[TaskStatus]int{
	TaskQueued:     1,
	TaskSent:       2,
	TaskInProgress: 3,
}

type TaskItem struct {
	Task  *Task
	Index int
}

type taskPriorityQueue []*TaskItem

func (tq taskPriorityQueue) Len() int {
	return len(tq)
}

func taskLess(i, j *Task) bool {
	p1, p2 := taskStatusPriority[i.Status], taskStatusPriority[j.Status]
	if p1 == p2 {
		return i.CreatedAt < j.CreatedAt
	}
	return p1 > p2
}

func (tq taskPriorityQueue) Less(i, j int) bool {
	return taskLess(tq[i].Task, tq[j].Task)
}

func (tq taskPriorityQueue) Swap(i, j int) {
	tq[i], tq[j] = tq[j], tq[i]
	tq[i].Index = i
	tq[j].Index = j

}

func (tq *taskPriorityQueue) Push(x any) {
	n := len(*tq)
	item := x.(*TaskItem)
	item.Index = n
	*tq = append(*tq, item)
}

func (tq *taskPriorityQueue) Pop() any {
	old := *tq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.Index = -1 // for safety
	*tq = old[0 : n-1]
	return item
}

func NewTaskQueue() TaskQueue {
	return TaskQueue{q: make(taskPriorityQueue, 0)}
}

type TaskQueue struct {
	q taskPriorityQueue
}

func (q *TaskQueue) Push(t *Task) {
	item := TaskItem{Task: t}
	heap.Push(&q.q, &item)
}

func (q *TaskQueue) Pop() *Task {
	item := heap.Pop(&q.q).(*TaskItem)
	return item.Task
}

func (q *TaskQueue) Peek() *Task {
	pk := q.Pop()
	q.Push(pk)
	return pk
}

func (q *TaskQueue) Size() int {
	return q.q.Len()
}

func (q *TaskQueue) Remove(index int) *Task {
	item := heap.Remove(&q.q, index).(*TaskItem)
	return item.Task
}

func (q *TaskQueue) RemoveById(taskId int64) *Task {
	for _, t := range q.q {
		if t.Task.TaskId == taskId {
			return q.Remove(t.Index)
		}
	}
	return nil
}
func (q *TaskQueue) GetTasks() []Task {
	var l []Task
	for _, t := range q.q {
		l = append(l, *t.Task)
	}
	return l
}
