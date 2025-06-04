package main

import (
	"fmt"
	"sync"
	"time"
)

var messages_to_uid_map map[int64]string

//var messages_to_uid_map_mu map[int64]sync.Mutex

var message_mutex sync.Mutex

func session(uid int64) {
	fmt.Println("uid", uid)
	for {
		message_mutex.Lock()
		if messages_to_uid_map[uid] != "-" {
			message := messages_to_uid_map[uid]

			fmt.Println("session id: ", uid)
			fmt.Println("message", message)

			//messages_mutex := messages_to_uid_map_mu[uid]
			messages_to_uid_map[uid] = "-"
		}
		message_mutex.Unlock()
	}
}

func main() {
	go Recv() // иницилиазируем ресв
	//messages_to_uid_map_mu = make(map[int64]sync.Mutex)
	messages_to_uid_map = make(map[int64]string)

	for {
		local_uid := inner_uid
		local_txt := inner_txtc
		go func() {
			if messages_to_uid_map[local_uid] == "" {
				fmt.Println("new session")
				go session(local_uid)
			}
		}()

		//messages_mutex := messages_to_uid_map_mu[local_uid]
		message_mutex.Lock()
		messages_to_uid_map[local_uid] = local_txt
		message_mutex.Unlock()

		time.Sleep(1000 * time.Millisecond)
	}
}
