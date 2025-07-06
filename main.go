package main

import (
	"encoding/base64"
	"encoding/gob"
	
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
)

type UserData struct {
	id             int
	name           string
	displayName    string
	about          string
	pfpURL         string
	verified       bool
	joinDate       string
	placeVisits    int
	followersCount int
	followingCount int
	friendCount    int
	friends        []int
}

func collectFor(userID int, browser *rod.Browser) {
	fmt.Printf("collecting for user %d\n", userID)

	var data UserData
	data.id = userID

	page := browser.MustPage(fmt.Sprintf("https://www.roblox.com/users/%d/profile", userID))

	//screenshot
	page.MustWaitStable().MustScreenshot(fmt.Sprintf("%d.png", userID))
	fmt.Println("screenshot")

	//check if user exists
	if page.MustHas(`#content > div > div > div.message-container > h3`) {
		fmt.Println("user does not exist")
		return
	}

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

	data.name = profileHeaderNames[1].MustText()
	data.displayName = profileHeaderTitle[0].MustText()
	data.verified = len(profileHeaderTitle) > 1
	data.friendCount, _ = strconv.Atoi(strings.Split(socialData[0].MustText(), " ")[0])
	data.followersCount, _ = strconv.Atoi(strings.Split(socialData[1].MustText(), " ")[0])
	data.followingCount, _ = strconv.Atoi(strings.Split(socialData[2].MustText(), " ")[0])
	
	if *aboutContainer.MustDescribe().ChildNodeCount > 0 {
		about := page.MustElementByJS(`() => document.querySelector(".profile-about-content-text")`)
		data.about = about.MustText()
	} else {
		data.about = ""
	}

	data.joinDate = joinDate.MustText()

	placeVisitsStr := strings.ReplaceAll(strings.Split(placeVisits.MustText(), " ")[0], ",", "")
	data.placeVisits, _ = strconv.Atoi(placeVisitsStr)

	pfpResource := pfpImage.MustResource()
	data.pfpURL = fmt.Sprintf("data:image/png;base64,%s", base64.StdEncoding.EncodeToString(pfpResource))

	fmt.Println(data)
}

func main() {
	//path, _ := launcher.LookPath()
	u := launcher.New().Headless(true).Leakless(false).MustLaunch()
	browser := rod.New().ControlURL(u).MustConnect()

	// load cookies
	if _, err := os.Stat("cookies.bin"); err == nil {
		cookieFile, err := os.Open("cookies.bin")
		if err != nil {
			panic(fmt.Sprintf("failed to open cookie file: %v", err))
		}
		defer cookieFile.Close()

		decoder := gob.NewDecoder(cookieFile)
		var cookies []*proto.NetworkCookie
		if err := decoder.Decode(&cookies); err != nil {
			panic(fmt.Sprintf("failed to load cookies: %v", err))
		}
		fmt.Println("cookies loaded from cookies.bin")

		browser.MustSetCookies(cookies...)
	}

	collectFor(1, browser)

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
}
