package main

import (
	"context"
	"fmt"
	cachekit "gitlab.services.mts.ru/jocasta/cache-kit"
	"log"
	"time"
)

func main() {
	c, err := cachekit.CreateCache(cachekit.Config{
		Type: "redis",
		//TTL: 3 * time.Second,
	})
	if err != nil {
		log.Fatal(err)
	}

	if err := c.SetValue(context.Background(), "a", 123); err != nil {
		log.Fatal(err)
	}
	res, err := c.GetValue(context.Background(), "a")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(res)

	time.Sleep(4 * time.Second)

	res, err = c.GetValue(context.Background(), "a")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(res)
}
