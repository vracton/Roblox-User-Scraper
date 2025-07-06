package main

import (
	"encoding/base64"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"

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

func collectFor(userID int, browser *rod.Browser) UserData {
	fmt.Printf("collecting for user %d\n", userID)

	var data UserData
	data.ID = userID

	page := browser.MustPage(fmt.Sprintf("https://www.roblox.com/users/%d/profile", userID))
	defer page.Close()

	//screenshot
	page.MustWaitStable() //.MustScreenshot(fmt.Sprintf("%d.png", userID))
	fmt.Println("screenshot")

	//check if user exists
	if page.MustHas(`#content > div > div > div.message-container > h3`) {
		fmt.Println("user does not exist")
		data.Exists = false
		return data
	}
	data.Exists = true

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
const UsersToCollect = 100
const NumWorkers = 10

func worker(id int, jobs <-chan int, results chan<- UserData, browserPool chan *rod.Browser) {
	for userID := range jobs {
		browser := <-browserPool
		userData := collectFor(userID, browser)
		browserPool <- browser
		results <- userData
	}
}

func main() {
	u := launcher.New().Headless(true).Leakless(false).MustLaunch()

	// create browser pool
	browserPool := make(chan *rod.Browser, NumWorkers)
	for i := 0; i < NumWorkers; i++ {
		browser := rod.New().ControlURL(u).MustConnect()

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

	// start
	for w := 1; w <= NumWorkers; w++ {
		go worker(w, jobs, results, browserPool)
	}

	// send jobs
	for i := StartID; i < StartID+UsersToCollect; i++ {
		jobs <- i
	}
	close(jobs)

	// collect
	users := make([]UserData, UsersToCollect)
	for i := 0; i < UsersToCollect; i++ {
		userData := <-results
		users[i] = userData
		fmt.Printf("Collected data for user %d (%d/%d)\n", userData.ID, i+1, UsersToCollect)
	}

	// clean up
	for i := 0; i < NumWorkers; i++ {
		browser := <-browserPool
		browser.MustClose()
	}

	// save
	json, _ := json.Marshal(users)
	os.WriteFile("out.json", json, 0644)
	fmt.Println("data saved to out.json")
}

//took avg of ~3.31 seconds per user with 10 workers
//took avg of 0.49 seconds per user with 100 workers (= 70+% cpu usage and 4gb ram :skull:)