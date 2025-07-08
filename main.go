package main

import (
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net/http"
	//"net/url"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

type UserData struct {
	ID             int    `json:"id"`
	Exists         bool   `json:"exists"`
	Name           string `json:"name"`
	DisplayName    string `json:"displayName"`
	About          string `json:"about"`
	PfpURL         string `json:"pfpURL"`
	Verified       bool   `json:"verified"`
	JoinDate       string `json:"joinDate"`
	PlaceVisits    int    `json:"placeVisits"`
	FollowersCount int    `json:"followersCount"`
	FollowingCount int    `json:"followingCount"`
	FriendCount    int    `json:"friendCount"`
	Friends        []int  `json:"friends"`
}

type FriendData struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	DisplayName string `json:"displayName"`
}

type FriendsResp struct {
	Data []FriendData `json:"data"`
}

func getFriends(userID int) ([]int, error) {
	url := fmt.Sprintf("https://friends.roblox.com/v1/users/%d/friends", userID)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get friends: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get friends, status code: %d", resp.StatusCode)
	}

	var friendsData FriendsResp
	if err := json.NewDecoder(resp.Body).Decode(&friendsData); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	friends := make([]int, len(friendsData.Data))
	for i, friend := range friendsData.Data {
		friends[i] = friend.ID
	}

	return friends, nil
}

func isBanned(userID int) bool {
	url := fmt.Sprintf("https://www.roblox.com/users/%d/profile", userID)
	resp, err := http.Get(url)
	if err != nil {
		return true
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusNotFound
}

var (
	numCollected int = 0
	collectedUsers []UserData
	usersMutex sync.Mutex
)

func collectFor(userID int, browser *rod.Browser) UserData {
	fmt.Printf("collecting for user %d\n", userID)

	var data UserData
	data.ID = userID

	//check if user exists
	banned := isBanned(userID)
	if banned {
		fmt.Println("user does not exist")
		data.Exists = false
		return data
	}

	page := browser.MustPage(fmt.Sprintf("https://www.roblox.com/users/%d/profile", userID))
	defer page.Close()

	//screenshot
	page.MustWaitStable() //.MustScreenshot(fmt.Sprintf("%d.png", userID))
	fmt.Println("screenshot")

	//secondary check if user exists (shouldn't be needed)
	if page.MustHas(`#content > div > div > div.message-container > h3`) {
		fmt.Println("user does not exist")
		data.Exists = false
		return data
	}
	data.Exists = true
	numCollected++

	//get data
	profileHeaderNames := page.MustElementsByJS(`() => document.querySelector(".profile-header-names").children`)
	fmt.Println("profile header names")
	profileHeaderTitle := page.MustElementsByJS(`() => document.querySelector(".profile-header-title-container").children`)
	fmt.Println("profile header title")
	socialData := page.MustElementsByJS(`() => document.querySelectorAll(".profile-header-social-count")`)
	fmt.Println("social data")
	aboutContainer := page.MustElementByJS(`() => document.querySelector("#about > div.section.profile-about.ng-scope > div:nth-child(2)")`)
	fmt.Println("about")
	pfpImage := page.MustElement(`#profile-header-container > div > div.avatar.avatar-headshot-lg.card-plain.profile-avatar-image > span > span > img`)
	fmt.Println("pfp image")
	page.Mouse.Scroll(0, 10000, 100) //scroll down so elements load
	joinDate := page.MustElementByJS(`() => document.querySelector("#profile-statistics-container > div > ul > li:nth-child(1) > span.MuiTypography-root.web-blox-css-tss-hzyup-Typography-body1-Typography-root.MuiTypography-inherit.web-blox-css-mui-clml2g > time")`)
	fmt.Println("join date")
	placeVisits := page.MustElement(`#profile-statistics-container > div > ul > li:nth-child(2) > span.MuiTypography-root.web-blox-css-tss-hzyup-Typography-body1-Typography-root.MuiTypography-inherit.web-blox-css-mui-clml2g`)
	fmt.Println("place visits")

	data.Name = profileHeaderNames[1].MustText()
	data.DisplayName = profileHeaderTitle[0].MustText()
	data.Verified = len(profileHeaderTitle) > 1
	data.FriendCount, _ = strconv.Atoi(strings.Split(socialData[0].MustText(), " ")[0])
	data.FollowersCount, _ = strconv.Atoi(strings.Split(socialData[1].MustText(), " ")[0])
	data.FollowingCount, _ = strconv.Atoi(strings.Split(socialData[2].MustText(), " ")[0])

	if *aboutContainer.MustDescribe().ChildNodeCount > 0 {
		about := page.MustElementByJS(`() => document.querySelector(".profile-about-content-text")`)
		data.About = about.MustText()
	} else {
		data.About = ""
	}

	data.JoinDate = joinDate.MustText()

	placeVisitsStr := strings.ReplaceAll(strings.Split(placeVisits.MustText(), " ")[0], ",", "")
	data.PlaceVisits, _ = strconv.Atoi(placeVisitsStr)

	pfpResource := pfpImage.MustResource()
	data.PfpURL = fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(pfpResource))
	data.Friends, _ = getFriends(userID)

	return data
}

const StartID = 1
const UsersToCollect = 500
const NumWorkers = 15

func worker(id int, jobs <-chan int, results chan<- UserData, browserPool chan *rod.Browser) {
	for userID := range jobs {
		browser := <-browserPool
		userData := collectFor(userID, browser)
		browserPool <- browser
		results <- userData
	}
}

func main() {
	command := os.Args[1]
	
	if command == "" || command == "collect" {
		//setup signal handling for no loss shutdown
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

		// create browser pool
		browserPool := make(chan *rod.Browser, NumWorkers)
		var browsers []*rod.Browser

		for i := 0; i < NumWorkers; i++ {
			u := launcher.New().Headless(true).Leakless(false).MustLaunch()
			browser := rod.New().ControlURL(u).MustConnect()
			browsers = append(browsers, browser)

			// load cookies
			if _, err := os.Stat("cookies.bin"); err == nil {
				cookieFile, err := os.Open("cookies.bin")
				if err != nil {
					panic(fmt.Sprintf("failed to open cookie file: %v", err))
				}

				decoder := gob.NewDecoder(cookieFile)
				var cookies []*proto.NetworkCookie
				if err := decoder.Decode(&cookies); err != nil {
					panic(fmt.Sprintf("failed to load cookies: %v", err))
				}
				cookieFile.Close()
				fmt.Printf("Browser %d: cookies loaded from cookies.bin\n", i)

				browser.MustSetCookies(cookies...)
			}

			browserPool <- browser
		}

		jobs := make(chan int, UsersToCollect)
		results := make(chan UserData, UsersToCollect)
		done := make(chan bool, 1)

		// start workers
		for w := 1; w <= NumWorkers; w++ {
			go worker(w, jobs, results, browserPool)
		}

		// send jobs in goroutine
		go func() {
			for i := StartID; i < StartID+UsersToCollect; i++ {
				jobs <- i
			}
			close(jobs)
		}()

		// collect results in goroutine
		go func() {
			users := make([]UserData, 0, UsersToCollect)
			collected := 0
			
			for userData := range results {
				users = append(users, userData)
				collected++
				fmt.Printf("collected data for user %d (%d/%d)\n", userData.ID, collected, UsersToCollect)
				
				// update global collected users for potential early save
				usersMutex.Lock()
				collectedUsers = append(collectedUsers[:0], users...)
				usersMutex.Unlock()
				
				if collected >= UsersToCollect {
					// save final data
					jsonData, _ := json.Marshal(users)
					os.WriteFile("500.json", jsonData, 0644)
					fmt.Println("data saved to 500.json")
					done <- true
					return
				}
			}
			done <- true
		}()

		//wait for completion or interrupt
		select {
		case <-done:
			fmt.Println("Collection completed successfully!")
		case <-sigChan:
			fmt.Println("\nReceived interrupt signal. Shutting down gracefully...")
			
			//give workers a moment to finish current tasks
			time.Sleep(2 * time.Second)
			
			//close results channel to stop collection
			close(results)
			
			//save partial data
			usersMutex.Lock()
			if len(collectedUsers) > 0 {
				jsonData, _ := json.Marshal(collectedUsers)
				filename := fmt.Sprintf("partial_%d_users.json", len(collectedUsers))
				os.WriteFile(filename, jsonData, 0644)
				fmt.Printf("Saved %d users to %s\n", len(collectedUsers), filename)
			}
			usersMutex.Unlock()
		}

		// Clean up all browsers
		fmt.Println("cleaning up browsers")
		for _, browser := range browsers {
			browser.MustClose()
		}

		fmt.Printf("collected data for %d valid users\n", numCollected)
		fmt.Println("Cleanup complete. Exiting...")
	} else if command == "trim" && len(os.Args[2]) > 5 {
		trim(os.Args[2])
	}
}

// save user cookies so that about me section can be accessed
// cookies := browser.MustGetCookies()
// fmt.Println(cookies)

// // save cookies to cookies.bin
// cookieFile, err := os.Create("cookies.bin")
// if err != nil {
// 	panic(fmt.Sprintf("Failed to create cookie file: %v", err))
// }
// defer cookieFile.Close()

// encoder := gob.NewEncoder(cookieFile)
// if err := encoder.Encode(cookies); err != nil {
// 	panic(fmt.Sprintf("Failed to save cookies: %v", err))
// }
// fmt.Println("cookies saved to cookies.bin")