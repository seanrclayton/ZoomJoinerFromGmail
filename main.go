package main

import (
	"context"
	"encoding/json"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/sqweek/dialog"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)

// Retrieve a token, saves the token, then returns the generated client.
func getClient(config *oauth2.Config) *http.Client {
	// The file token.json stores the user's access and refresh tokens, and is
	// created automatically when the authorization flow completes for the first
	// time.
	tokFile := "token.json"
	tok, err := tokenFromFile(tokFile)
	if err != nil {
		tok = getTokenFromWeb(config)
		saveToken(tokFile, tok)
	}
	return config.Client(context.Background(), tok)
}

// Request a token from the web, then returns the retrieved token.
func getTokenFromWeb(config *oauth2.Config) *oauth2.Token {
	authURL := config.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
	fmt.Printf("Go to the following link in your browser then type the "+
		"authorization code: \n%v\n", authURL)

	var authCode string
	if _, err := fmt.Scan(&authCode); err != nil {
		log.Fatalf("Unable to read authorization code: %v", err)
	}

	tok, err := config.Exchange(context.TODO(), authCode)
	if err != nil {
		log.Fatalf("Unable to retrieve token from web: %v", err)
	}
	return tok
}

// Retrieves a token from a local file.
func tokenFromFile(file string) (*oauth2.Token, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

// Saves a token to a file path.
func saveToken(path string, token *oauth2.Token) {
	fmt.Printf("Saving credential file to: %s\n", path)
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		log.Fatalf("Unable to cache oauth token: %v", err)
	}
	defer f.Close()
	json.NewEncoder(f).Encode(token)
}

type MeetingInfo struct{
	time float64
	summary string
	zoomLink string
}

func getEvents() map[float64]MeetingInfo {

	meetings := make(map[float64]MeetingInfo)

	ctx := context.Background()
	b, err := ioutil.ReadFile("credentials.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	// If modifying these scopes, delete your previously saved token.json.
	config, err := google.ConfigFromJSON(b, calendar.CalendarReadonlyScope)
	if err != nil {
		log.Fatalf("Unable to parse client secret file to config: %v", err)
	}
	client := getClient(config)

	srv, err := calendar.NewService(ctx, option.WithHTTPClient(client))
	if err != nil {
		log.Fatalf("Unable to retrieve Calendar client: %v", err)
	}

	t := time.Now().Format(time.RFC3339)
	events, err := srv.Events.List("primary").ShowDeleted(false).
		SingleEvents(true).TimeMin(t).MaxResults(10).OrderBy("startTime").Do()
	if err != nil {
		log.Fatalf("Unable to retrieve next ten of the user's events: %v", err)
	}
	fmt.Println("Upcoming events:")
	log.Info("Upcoming Events:")
	if len(events.Items) == 0 {
		fmt.Println("No upcoming events found.")
		log.Info("NO upcoming events found")
	} else {
		for _, item := range events.Items {
			date := item.Start.DateTime
			if date == "" {
				date = item.Start.DateTime
			}

			parsedDate, _ := time.Parse(time.RFC3339,date)
			currentTime  := time.Now()
			var zoomLink string = ""
			if item.ConferenceData == nil {
				zoomLink = parseMeetingDescription(item.Description)
			}else{
				zoomLink = item.ConferenceData.EntryPoints[0].Label
				}
			timeTillMeeting := parsedDate.Sub(currentTime).Minutes()
			meetings[timeTillMeeting] = MeetingInfo{
				time: timeTillMeeting,
				summary:  item.Summary,
				zoomLink: zoomLink,
			}

		}
	}
	return meetings
}

func parseMeetingDescription(description string) string{
	var zoomLink string
	for _, element := range strings.Split(description, " ") {
		if strings.ContainsAny(element,"zoom") && strings.ContainsAny("pwd=",element){
			r, _ := regexp.Compile("href=(.*)>")


			thing := r.FindString(element)
			// XXXXXX below in the href prefex check needs to be replaced with your company name or personal zoom
			if strings.HasPrefix(thing,"href=\"https://XXXXXXXXXX.zoom.us") && strings.ContainsAny(thing,"?"){
				removeHref := strings.TrimPrefix(thing,"href=")
				zoomLinkWithEnding := strings.SplitAfter(removeHref,">")[0]
				zoomLink = strings.ReplaceAll(zoomLinkWithEnding,">","")

			}else{
				zoomLink = "none"
			}
		}

	}
	return zoomLink
}

func openBrowser(url string) bool {
	// TODO change method to support other OS, see bottom code 
	err := exec.Command("xdg-open", url).Start()
	if err != nil {
		log.Fatal(err)
	}
	return true
	//var args []string
	//switch runtime.GOOS {
	//case "darwin":
	//	args = []string{"open"}
	//case "windows":
	//	args = []string{"cmd", "/c", "start"}
	//default:
	//	args = []string{"xdg-open"}
	//}
	//cmd := exec.Command(args[0], append(args[1:], url)...)
	//return cmd.Start() == nil
}

func main(){
	logName := fmt.Sprintf("zoomJoiner%v.log",time.Now().Day())
	file, err := os.OpenFile(logName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		log.Fatal(err)
	}

	log.SetOutput(file)
	for _,element := range getEvents(){
		if element.time < 60.02 &&  element.zoomLink != "none" {
			fmt.Println(element.time)
			log.Info("Time till meeting:")
			log.Info(element.time)

			log.Info("Meeting Name:")
			fmt.Println(element.summary)
			log.Info(element.summary)

			log.Info("Zoom Link:")
			log.Info(element.zoomLink)
			fmt.Println(element.zoomLink)
			ok := dialog.Message("joining %v zoom meeting early", element.summary).YesNo()
			if ok == true {
				openBrowser("https://"+element.zoomLink)
				break

			}else{
				// TODO adjust the time - I imagine 30 is good.
				fmt.Println("No was pressed, sleeping for 30 minutes")
				log.Info("No was pressed, sleeping for 30 minutes")
				time.Sleep(1800)
				}

			}
		}

	}



