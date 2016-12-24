// http://askubuntu.com/a/50000 was a great help to cover edge cases
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os/exec"
	"strconv"
	"strings"
)

/*
	USAGE: sudo ./shownow -user=testuser -key="ssh public key here"
*/

// If an error exists, print message and panic
func checkError(msg string, err error) {
	if err != nil {
		fmt.Println(msg)
		panic(err)
	}
}

func main() {
	whoami := "faraz" // user.Current doesn't work when cross-compiling macOS -> Linux
	userFlag := flag.String("user", "", "Username")
	keyFlag := flag.String("key", "", "Public Key")

	flag.Parse()

	user := *userFlag
	key := *keyFlag

	ip := getIP()
	port := getFreePort()

	users, err := exec.Command("users").Output()
	checkError("Error getting users", err)

	// Check if user exists
	if strings.Contains(user, "root") || strings.Contains(string(users), user) {
		fmt.Println("User already exists")
		fmt.Println("Flag passed: " + user)
		fmt.Println(string(users))
		return
	}

	// Add user
	_, err = exec.Command("/bin/sh", "-c", "sudo /usr/sbin/adduser --disabled-password --gecos \"\" "+user).Output()
	checkError("Error creating user", err)
	fmt.Printf("Added user: %s\n", user)

	// Modify .ssh externally
	_, err = exec.Command("/bin/sh", "-c", "cd /home/"+user+" && sudo mkdir .ssh && sudo chown -R "+user+":"+user+" .ssh "+" && cd .ssh && touch authorized_keys && sudo chown "+whoami+":"+whoami+" authorized_keys").Output()
	checkError("Error modifying .ssh", err)
	fmt.Println("Created .ssh skeleton")

	// Add SSH key & prevent user from running other commands
	// Fix " right before key
	reversecommand := `ssh ` + user + `@` + ip + ` -N -R ` + port + `:localhost:` + port
	command := `cd /home/` + user + `/.ssh/` + ` && sudo echo "command=\"SHELL=/bin/false && printf 'You cannot login. To tunnel, use the following:\n` + reversecommand + `\n'\",no-agent-forwarding,no-X11-forwarding,permitopen=\"localhost:` + port + `\" ` + key + `" > ` + `authorized_keys && sudo chown ` + user + ":" + user + ` authorized_keys`

	_, err = exec.Command("/bin/sh", "-c", command).Output()
	checkError("Error restricting .ssh to tunnel only", err)
	fmt.Printf("Restricted %s tunneling ability for port %s only\n", user, port)

	tld = "ml"                            // ml or cf
	subdomain := user + ".shownow." + tld // TODO NGINX config
	fmt.Printf("alias shownow=%s && open %s\n", reversecommand, subdomain)
}

func getIP() string {
	resp, err := http.Get("https://api.ipify.org")
	checkError("Error getting IP", err)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	return string(body)
}

// https://github.com/phayes/freeport/blob/master/freeport.go
// https://api.ipify.org - better than myexternalip
func getFreePort() string {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	checkError("resolve", err)

	l, err := net.ListenTCP("tcp", addr)
	checkError("listen", err)

	defer l.Close()
	free := strconv.Itoa(l.Addr().(*net.TCPAddr).Port)
	fmt.Println("Port: " + free)
	if len(free) < 4 {
		panic("Error getting port")
	}
	return free
}
