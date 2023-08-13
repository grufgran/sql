package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"unicode"

	"github.com/eiannone/keyboard"
	"github.com/grufgran/config"
	term "github.com/grufgran/go-terminal"
	"github.com/grufgran/termMenu"
)

type menuDataProvider struct {
	conf              *config.Config
	menu              *termMenu.TermMenu
	menuItems         []termMenu.MenuItem
	allChoices        map[string]menuChoice
	subMenuContainers map[string]int
	initChoice        string
	showPwd           int
	filterStr         string
	filterMatches     map[string]bool
}

func newMenuDataProvider(conf *config.Config) *menuDataProvider {
	mdp := menuDataProvider{
		conf:              conf,
		menuItems:         make([]termMenu.MenuItem, 0),
		allChoices:        make(map[string]menuChoice),
		subMenuContainers: make(map[string]int),
	}

	// get last choices
	lastSect := conf.PropOrDefault("settings", "last_sect", "")
	lastUser := conf.PropOrDefault("settings", "last_user", "")

	// create all menuOptions
	mdp.createMenuItems(lastSect, lastUser)

	return &mdp
}

func (mdp *menuDataProvider) receiveMenu(menu *termMenu.TermMenu) {
	mdp.menu = menu
}

// provide menu header
func (mdp *menuDataProvider) ProvideMenuHeader(menuId string) []termMenu.StyledText {
	headerText := "Choose database login:"
	//headerText = fmt.Sprintf("Choose database login:    (%v)", time.Now())
	//headerText = "Choose database login: (2023-03-19 07:37:20.512914612 +0100 CET m=+11.035318807)"
	header := termMenu.NewStyledText(headerText, term.ForegroundCyan)
	return []termMenu.StyledText{header}
}

// Provide last saved choice
func (mdp *menuDataProvider) ProvideInitialChoice(menuId string) string {
	return mdp.initChoice
}

// provide menu items
func (mdp *menuDataProvider) ProvideMenuItems(menuId string) []termMenu.MenuItem {
	if menuId == "main" {
		return mdp.menuItems
	}
	return []termMenu.MenuItem{}
}

func (mdp *menuDataProvider) AboutToDisplayMenuItem(menuId, itemId string, styledText termMenu.StyledText) []termMenu.StyledText {
	if menuId == "main" {
		if containerIndex, exists := mdp.subMenuContainers[itemId]; exists && !mdp.menuItems[containerIndex].IsExpanded {
			for _, subItem := range mdp.menuItems[containerIndex].SubItems() {
				for matchId := range mdp.filterMatches {
					if matchId == subItem.Id() {
						return styledText.HilightText("+", false, term.ForegroundMagenta)
					}
				}
			}
		} else {
			return styledText.HilightText(mdp.filterStr, false, term.ForegroundMagenta)
		}
	}
	return []termMenu.StyledText{styledText}
}

func (mdp *menuDataProvider) ProvideMenuFooter(menuId, chosenId string) []termMenu.StyledText {
	if menuOption, exists := mdp.allChoices[chosenId]; exists {
		footerText := menuOption.execStr[mdp.showPwd]
		//footerText := fmt.Sprintf("%v, num_rows: %v", menuOption.execStr[mdp.showPwd], mdp.menu.NumDisplayedRows())
		footer := termMenu.NewStyledText(footerText)
		if len(mdp.filterStr) > 0 {
			filterLabel, filterText := createFilterStyledText("Filter: ", mdp.filterStr, "\n")
			return []termMenu.StyledText{filterLabel, filterText, footer}
		} else {
			return []termMenu.StyledText{footer}
		}
	} else {
		if len(mdp.filterStr) > 0 {
			filterLabel, filterText := createFilterStyledText("Filter: ", mdp.filterStr)
			return []termMenu.StyledText{filterLabel, filterText}
		} else {
			/*
				footerText := fmt.Sprintf("num_rows: %v", mdp.menu.NumDisplayedRows())
				footer := termMenu.NewStyledText(footerText)
				return []termMenu.StyledText{footer}
			*/
			return []termMenu.StyledText{}
		}
	}
}

func (mdp *menuDataProvider) HandleKeyPress(menuId, chosenId string, ch rune, key keyboard.Key) termMenu.MenuTask {
	response := termMenu.DoNothing
	if menuId == "main" {
		if key == keyboard.KeyCtrlS {
			mdp.showPwd = 1 - mdp.showPwd
			response = termMenu.RerenderMenu
		} else if key == keyboard.KeyBackspace || key == keyboard.KeyBackspace2 || key == keyboard.KeyDelete {
			if len(mdp.filterStr) > 0 {
				mdp.filterStr = mdp.filterStr[:len(mdp.filterStr)-1]
				if len(mdp.filterStr) > 0 {
					mdp.filterMatches = mdp.menu.FindSubString(mdp.filterStr, false)
				} else {
					mdp.filterMatches = make(map[string]bool)
				}
				response = termMenu.RerenderMenu
			}
		} else if unicode.IsPrint(ch) {
			mdp.filterMatches = mdp.menu.FindSubString(mdp.filterStr+string(ch), false)
			if len(mdp.filterMatches) > 0 {
				mdp.filterStr += string(ch)
				response = termMenu.RerenderMenu
				if len(mdp.filterStr) > 2 {
					for itemId, isExpanded := range mdp.filterMatches {
						if !isExpanded {
							mdp.menuItems[mdp.allChoices[itemId].topLevelIndex].IsExpanded = true
							response = termMenu.RedisplayMenu
							mdp.initChoice = chosenId
						}
					}
				}
			}
		}
	}
	return response
}

func (mdp *menuDataProvider) TickerEvent() termMenu.MenuTask {
	return termMenu.RerenderMenu
}

func (mdp *menuDataProvider) ReceiveExpandedStatus(menuId, chosenId string, isExpanded bool) {
	if menuId == "main" {
		mdp.menuItems[mdp.subMenuContainers[chosenId]].IsExpanded = isExpanded
	}
}

func (mdp *menuDataProvider) IsMenuItemExpanded(menuId string, menuItemId string) bool {
	if menuId == "main" {
		return mdp.menuItems[mdp.allChoices[menuItemId].topLevelIndex].IsExpanded
	}
	return false
}

func (mdp *menuDataProvider) MenuDisplayed(menuId string, reason termMenu.MenuDisplayedReason) {
	//fmt.Print("len:", len(mdp.menu.RowLengths), "pos:", mdp.menu.RowLengthsIndex)
}

func (mdp *menuDataProvider) createMenuItems(lastSect string, lastUser string) {
	// Get all sectnames from conf
	sectNames := mdp.conf.SectNames()

	// loop thru all sections, build connection string and add to menu
	id := 0
	key := ""
	mdp.initChoice = "0"
	topLevelIndex := -1
	for _, sectName := range sectNames {
		if sectName != "settings" {
			sect := mdp.conf.Sect(sectName)
			active, exists := sect.PropVal("active")
			if exists && active == "0" {
				continue
			}
			topLevelIndex++
			// there can be multiple users defined
			if user, isDefined := sect.PropVal("user"); isDefined {
				users := strings.Split(user, ",")
				isSubItem := false
				if len(users) > 1 {
					isSubItem = true
					mdp.menuItems = append(mdp.menuItems, termMenu.NewMenuItem(sectName, sectName, false))
					mdp.subMenuContainers[sectName] = topLevelIndex

				}
				for _, user := range users {
					pwd := getPwdForUser(sect, user)
					execStrMaskedPwd := buildExecStr(sect, user, "***")
					execStrRevealedPwd := buildExecStr(sect, user, pwd)
					key, id = generateMenuChoiceKey(id)
					// add to map
					mo := newMenuChoice(sectName, user, topLevelIndex, [2]string{execStrMaskedPwd, execStrRevealedPwd})
					mdp.allChoices[key] = mo
					// add to menuItems
					if isSubItem {
						mdp.menuItems[topLevelIndex].AddSubItem(termMenu.NewMenuItem(user, key, true))
					} else {
						mdp.menuItems = append(mdp.menuItems, termMenu.NewMenuItem(sectName, key, true))
					}
					// set lastChoice
					if sectName == lastSect && user == lastUser {
						mdp.initChoice = key
						if isSubItem {
							mdp.menuItems[topLevelIndex].IsExpanded = true
						}
					}
				}
			} else {
				execStr := buildExecStr(sect, "", "")
				mo := newMenuChoice(sectName, "", topLevelIndex, [2]string{execStr, execStr})
				key, id = generateMenuChoiceKey(id)
				// add to map
				mdp.allChoices[key] = mo
				mdp.menuItems = append(mdp.menuItems, termMenu.NewMenuItem(sectName, key, true))
				if lastSect == sectName {
					mdp.initChoice = key
				}
			}
		}
	}
}

func buildExecStr(sect *config.Sect, user string, pwd string) string {
	execType, exists := sect.PropVal("exe")
	if execType == "psql" || !exists {
		return buildPgStr(sect, user, pwd)
	}

	return "not implemented"
}

func buildPgStr(sect *config.Sect, user string, pwd string) string {
	// psql postgresql://[user[:password]@][host][:port][,...][/dbname][?param1=value1&...]
	var sb strings.Builder
	sb.WriteString("psql postgresql://")

	// add user and password, if present
	if _, exists := sect.PropVal("user"); exists {
		sb.WriteString(user)
		if _, exists := sect.PropVal("pwd"); exists {
			sb.WriteString(":")
			sb.WriteString(pwd)
		}
		sb.WriteString("@")
	}

	// add host and port, if present
	if val, exists := sect.PropVal("host"); exists {
		sb.WriteString(val)
		if val, exists := sect.PropVal("port"); exists {
			sb.WriteString(":")
			sb.WriteString(val)
		}
	}

	// add db, if present
	if val, exists := sect.PropVal("db"); exists {
		sb.WriteString("/")
		sb.WriteString(val)
	}

	return sb.String()
}

func getPwdForUser(sect *config.Sect, searchedUser string) string {
	if pwd, isDefined := sect.PropVal("pwd"); isDefined {
		pwds := strings.Split(pwd, ",")
		if user, isDefined := sect.PropVal("user"); isDefined {
			users := strings.Split(user, ",")
			for num, user := range users {
				if user == searchedUser {
					if num > len(pwds)-1 {
						return pwds[len(pwds)-1]
					} else {
						return pwds[num]
					}
				}
			}
		}
	}
	return ""
}

// return a key for the menuOption map
func generateMenuChoiceKey(id int) (string, int) {
	key := strconv.Itoa(id)
	id++
	return key, id
}

// save chosen id to config file
func (mdp *menuDataProvider) saveChoice(lastId string, confFile string) {
	// get sect and user for chosen id
	mo := mdp.allChoices[lastId]
	fileContent, err := ioutil.ReadFile(confFile)
	if err != nil {
		fmt.Printf("Could not read configfile: %v", confFile)
		return
	}

	lines := strings.Split(string(fileContent), "\n")

	for i, line := range lines {
		if strings.HasPrefix(line, "last_sect=") {
			lines[i] = fmt.Sprintf("last_sect=%v", mo.sectName)
		}
		if strings.HasPrefix(line, "last_user=") {
			lines[i] = fmt.Sprintf("last_user=%v", mo.user)
		}
	}
	output := strings.Join(lines, "\n")
	err = ioutil.WriteFile(confFile, []byte(output), 0644)
	if err != nil {
		fmt.Printf("Could not save last_choice to %v", confFile)
	}
}

func (mdp *menuDataProvider) startProgram(chosenId string) {
	progName, params := mdp.buildExecData(chosenId)
	cmd := exec.Command(progName, params...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Run()
}

// slice execStr and return the first item as string (i e the program name) and the rest as a slice (i e program params)
func (mdp *menuDataProvider) buildExecData(chosenId string) (string, []string) {
	// get the exec str where password is shown for the chosen id
	mo := mdp.allChoices[chosenId]
	execStr := mo.execStr[1]
	items := strings.SplitN(execStr, " ", 2)
	programName := items[0]
	params := strings.Split(items[1], " ")
	return programName, params
}

func createFilterStyledText(prefix, filterStr string, postfixes ...string) (termMenu.StyledText, termMenu.StyledText) {
	filterLabel := termMenu.NewStyledText(prefix)
	var sb strings.Builder

	sb.WriteString(filterStr)
	if len(filterStr) < 3 {
		sb.WriteString(strings.Repeat("-", 3-len(filterStr)))
	}
	for _, postfix := range postfixes {
		sb.WriteString(postfix)
	}
	filterText := termMenu.NewStyledText(sb.String(), term.ForegroundMagenta)
	return filterLabel, filterText
}
