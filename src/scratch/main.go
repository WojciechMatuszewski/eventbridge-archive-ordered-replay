package main

import (
	"app/env"
	"fmt"
)

func main() {
	run()
}

func run() (err error) {
	_, err = env.Get(env.EVENT_BUS_ARCHIVE_NAME)
	t(&err)

	return err
}

func t(err *error) {
	var other error
	fmt.Println(err, err == nil, other, &other, other == nil)
}
