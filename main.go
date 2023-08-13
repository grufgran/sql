package main

import (
	"fmt"
	"os"

	"github.com/grufgran/config"
	"github.com/grufgran/termMenu"
)

func main() {
	// read config-file
	conf, err := config.NewConfigFromFile(nil, "sql.conf", nil)
	if err != nil {
		fmt.Printf("err: %v\n", err)
		os.Exit(1)
	}

	// create menuDataProvier
	mdp := newMenuDataProvider(conf)

	// create menu
	menu := termMenu.NewTermMenu("main", mdp)

	// send menu to menuDataProvider
	mdp.receiveMenu(menu)

	// show menu
	menu.Display("main")
	//menu.StartTicker(2)

	// wait for the user to choose.
	for {
		menuId, selectedId := menu.WaitForDecision()
		if menuId == "main" {
			if selectedId != "" {
				// save selectedId. It will be the initial selected item next time, menu is shown
				mdp.saveChoice(selectedId, "sql.conf")

				// start the program
				mdp.startProgram(selectedId)
			}
			break
		}
	}
}
