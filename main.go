// http://askubuntu.com/a/50000 was a great help to cover edge cases
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// If an error exists, print message and panic
func checkError(msg string, err error) {
	if err != nil {
		fmt.Println(msg)
		panic(err)
	}
}

func main() {

	// Shell
	sh := "/bin/sh"

	// Comamnd flag
	c := "-c"

	whoamiFlag := flag.String("whoami", "faraz", "Current Username") // user.Current doesn't work when cross-compiling macOS -> Linux
	userFlag := flag.String("user", "", "Username")
	keyFlag := flag.String("key", "", "Public Key")
	tldFlag := flag.String("tld", "ml", "TLD (ml/cf)")
	tunnelOfflineMessageFlag := flag.String("tunneloffline", "Tunnel offline", "Tunnel offline message to show on a 502 error.")

	flag.Parse()

	whoami := *whoamiFlag
	user := *userFlag
	key := *keyFlag
	tld := *tldFlag
	tunnelOfflineMessage := *tunnelOfflineMessageFlag

	// Validate user & key flags
	if len(user) < 2 || len(key) < 10 {
		fmt.Println("Please pass in all required arguments")
		return
	}

	// Get listed users, home directory listings, and NGINX configurations
	listedUsers, err := exec.Command(sh, c, "sudo cat /etc/passwd").Output()
	homeDirs, err := exec.Command(sh, c, "sudo ls /home").Output()
	nginxConfigs, err := exec.Command(sh, c, "sudo ls /etc/nginx/sites-enabled/*").Output()
	checkError("Error getting users", err)

	// Combine all together to check for collisions
	users := string(listedUsers) + string(homeDirs) + string(nginxConfigs)

	// Check if user exists
	if strings.Contains(user, "root") || strings.Contains(string(users), user) {
		fmt.Printf("%s already exists\n", user)
		return
	}

	// Starting border
	fmt.Println("--------------------------------------------")

	// Get IP and free port
	ip := getIP()
	port := getFreePort()

	// Add user
	addUserCommand := fmt.Sprintf(`sudo /usr/sbin/adduser --disabled-password --gecos "" %s`, user)
	_, err = exec.Command(sh, c, addUserCommand).Output()
	checkError("Error creating user", err)
	fmt.Printf("Added user: %s\n", user)

	// Modify .ssh externally
	modifySSHCommand := fmt.Sprintf(`cd /home/%s && mkdir .ssh && sudo chown -R %s:%s .ssh && cd .ssh && touch authorized_keys && sudo chown %s:%s authorized_keys`, user, user, user, whoami, whoami)
	_, err = exec.Command(sh, c, modifySSHCommand).Output()
	checkError("Error modifying .ssh", err)
	fmt.Println("Created .ssh skeleton")

	// Add SSH key & prevent user from running other commands
	reverseCommand := fmt.Sprintf(`ssh %s@%s -N -R PORT:localhost:%s`, user, ip, port)
	restrictSSHCommand := fmt.Sprintf(`cd /home/%s/.ssh && sudo echo "command=\"SHELL=/bin/false && printf 'You cannot login. To tunnel, use the following:\n%s and replace PORT with your local port.\n'\",no-agent-forwarding,no-X11-forwarding,permitopen=\"localhost:%s\" %s" > authorized_keys && sudo chown %s:%s authorized_keys`, user, reverseCommand, port, key, user, user)
	_, err = exec.Command(sh, c, restrictSSHCommand).Output()
	checkError("Error restricting .ssh to tunnel only", err)
	fmt.Printf("Restricted %s tunneling ability for port %s only\n", user, port)

	subdomain := user + ".shownow." + tld

	// Add NGINX configuration for reverse proxying subdomain
	nginxConfig, err := os.Create("/etc/nginx/sites-enabled/" + user + ".conf")
	checkError("Error creating NGINX configuration", err)
	nginxConf := fmt.Sprintf("server { listen 80; server_name %s.shownow.ml www.%s.shownow.ml; location / { proxy_pass http://localhost:%s; proxy_set_header Host $host; proxy_set_header X-Real-IP $remote_addr; proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for; proxy_set_header X-Forwarded-Proto $scheme; } error_page 502 /502.html; location = /502.html { return 502 '%s'; add_header Content-Type text/plain; } }", user, user, port, tunnelOfflineMessage)
	nginxConfig.WriteString(nginxConf)
	nginxConfig.Sync()

	err = exec.Command(sh, c, "sudo systemctl reload nginx").Start()
	checkError("Error reloading NGINX", err)

	fmt.Printf("NGINX reloaded\n\n")
	fmt.Printf("Alias/function to start tunneling locally:\n\n")

	// Configured port alias - modify PORT & add to shell rc, then simply run: shownow
	fmt.Printf("alias shownow=\"%s && open %s\"\n\n", reverseCommand, subdomain)

	// Function which takes a port number and sets up the tunnel - add to shell rc, and run like so: shownow 1000
	fmt.Printf(`shownow() { ssh %s@%s -N -R $1\:localhost:%s; }`+"\n\n", user, ip, port)
}

// https://api.ipify.org - better than myexternalip
func getIP() string {
	resp, err := http.Get("https://api.ipify.org")
	checkError("Error getting IP", err)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	return string(body)
}

// https://github.com/phayes/freeport/blob/master/freeport.go
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
