package main

import (
	"fmt"

	"github.com/Crypto89/vulcan/config"
	"github.com/davecgh/go-spew/spew"
	log "github.com/sirupsen/logrus"
)

func main() {
	log.SetLevel(log.DebugLevel)

	// facts, err := facter.New()
	// if err != nil {
	// 	log.Fatalf("error parsing facts: %s", err)
	// }

	// b, err := json.Marshal(facts)
	// if err != nil {
	// 	log.Fatalf("error marshalling facts: %s", err)
	// }

	// fmt.Printf("%s\n", b)

	cfg, err := config.LoadDir("/app/test")
	if err != nil {
		log.Fatalf("%s", err)
	}

	fmt.Println(spew.Sdump(cfg))
}
