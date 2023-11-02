package main

import (
	"log"

	"github.com/marf41/artnet"
)

func main() {
	for {
		an, err := artnet.GetAndParse(true)
		if err != nil {
			log.Println(err)
		} else {
			if an.HasChannels {
				log.Println(an.ChannelsAsString(1, 16, " | "))
			}
			ch, ok := an.Channel(1)
			if ok {
				log.Printf("First channel: %d\n", ch)
			} else {
				log.Printf("No channels.")
			}
		}
	}
}
