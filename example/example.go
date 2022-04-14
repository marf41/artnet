package main

import (
  "log"
  "github.com/marf41/artnet"
)

func main() {
  for {
    an, err := artnet.GetAndParse(true)
    if err != nil { log.Println(err) }
    log.Println(an.Channels(1, 16, " | "))
    log.Printf("First channel: %d\n", an.Data[1])
  }
}
