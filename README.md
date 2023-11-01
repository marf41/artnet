# ArtNet

Simple Art-Net parser for Golang. WIP.

Parses ArtDmx, ArtPoll, and ArtPollReply packets, receive-only

Tested using Chamsys MagicQ for PC.

## Windows fix

Add following line to `go.mod`:
```
replace github.com/projecthunt/reuseable => github.com/xmapst/reuseable v0.0.0-20220729041713-16fb23d1c9ef
```
