package utils

import `time`

type Message struct {
	Topic   string `json:"topic" binding:"required"`
	Message string `json:"message" binding:"required"`
}

type Messages struct {
	Topic string `json:"topic" binding:"required"`
}

func TimeConnect(fn func() error, attempts int, delay time.Duration) (err error) {
	for attempts > 0 {
		if err = fn(); err == nil {
			time.Sleep(delay)
			attempts--

			continue
		}
		return nil
	}
	return
}
