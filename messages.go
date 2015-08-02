package main

import (
	"bufio"
	"io"
	"log"
	"strings"
)

func Prompt(rw io.ReadWriter, question string) (string, error) {
	bufw := bufio.NewWriter(rw)
	bufr := bufio.NewReader(rw)

	if _, err := bufw.WriteString(question); err != nil {
		log.Printf("An error occured writing: %s\n", err.Error())
		return "", err
	}

	if err := bufw.Flush(); err != nil {
		log.Printf("An error occured flushing: %s\n", err.Error())
		return "", err
	}

	ans, err := bufr.ReadString('\n')
	if err != nil {
		log.Printf("An error occured reading: %s\n", err.Error())
		return "", err
	}
	ans = strings.TrimSpace(ans)

	return ans, nil
}

const INTRO_MSG string = string(`A monolithic building appears before you. You have arrived at the office. Try
not to act suspicious.
`)

const NICK_MSG string = "Enter a nickname:\n"

const ROOM_MSG string = "Log in to your team's assigned collaboration channel:\n"

const FULL_MSG string = "It seems your teammates have started without you. Exiting..."

const START_MSG string = string(`* -- | Everyone has arrived, mission starting...
* -- | Ask for /help to get familiar around here
`)

const FAIL_MSG string = string(`fail | You wake up bleary eyed and alone in a concrete box. Your head has a
fail | lump on the side. It seems corporate security noticed you didn't belong,
fail | you should have acted faster. You wonder if you will ever see your
fail | burrow again
`)

const HELP_MSG string = string(`help -- |  Usage:
help -- |
help -- |     /[cmd] [arguments]
help -- |
help -- |  Available commands:
help -- |
help -- |    /msg [to] [text]         send message to coworker
help -- |    /list                    look at files you have access to
help -- |    /send [to] [filename]    move file to coworker
help -- |    /look                    show coworkers
`)

const GLENDA_MSG string = string(`Glenda | Psst, hey there. I'm going to need your help if we want to exfiltrate
Glenda | these documents. You have clearance that I don't.
Glenda |
Glenda | You each have access to a different set of sensitive files. Within your
Glenda | group you can freely send files to each other for further analysis.
Glenda | However, when sending files to me, the corporate infrastructure team
Glenda | will be alerted if you exceed your transfer quota. Working on too many
Glenda | files will make them suspicious.
Glenda |
Glenda | Please optimize your transfers by the political impact it will create
Glenda | without exceeding any individual transfer quota. The file's security
Glenda | clearance is a good metric to go by for that. Thanks!
Glenda |
Glenda | When each of you is finished sending me files, send me the message
Glenda | 'done'. I'll wait to hear this from all of you before we execute phase
Glenda | two.
`)
