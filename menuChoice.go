package main

type menuChoice struct {
	sectName      string
	user          string
	topLevelIndex int
	execStr       [2]string
}

func newMenuChoice(sectName, user string, topLevelIndex int, execStr [2]string) menuChoice {
	mo := menuChoice{
		sectName:      sectName,
		user:          user,
		topLevelIndex: topLevelIndex,
		execStr:       execStr,
	}
	return mo
}
