package main

import (
	"encoding/json"
	"fmt"
	"log"
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/joho/godotenv"
)

type UserCreatedResult = int

const (
	DatesFile                                = "dates.json"
	DateFormat                               = "2006-01-02"
	UserCreatedBefore      UserCreatedResult = 0
	UserCreatedWithinRange UserCreatedResult = 1
	UserCreatedWithinDay   UserCreatedResult = 2
)

var (
	botToken            string
	alertsChannelID     string
	recentJoinChannelID string
	allowedRole         string
	dates               []string
)

func main() {
	loadEnvironment()
	loadDates()

	dg, err := discordgo.New("Bot " + botToken)
	if err != nil {
		log.Panic(err)
	}

	// Handle the member join event
	dg.AddHandler(onMemberAdd)

	// Handle message event
	dg.AddHandler(onMessage)

	// Set the intents correctly
	dg.Identify.Intents = discordgo.IntentsMessageContent | discordgo.IntentsGuildMembers | discordgo.IntentsGuildMessages

	// Open a websocket connection to Discord and begin listening.
	err = dg.Open()
	if err != nil {
		log.Panic(err)
		return
	}

	// Wait here until CTRL-C or other term signal is received.
	slog.Info("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt)
	<-sc

	// Cleanly close down the Discord session.
	dg.Close()
}

func loadEnvironment() {
	// Read dotenv, ignore any failures because we only need to do this on dev
	godotenv.Load()

	// Important configuration variables, including the bot token and various discord snowflakes
	botToken = os.Getenv("BOT_TOKEN")
	alertsChannelID = os.Getenv("CHANNEL_ID")
	recentJoinChannelID = os.Getenv("RECENT_JOIN_CHANNEL_ID")
	allowedRole = os.Getenv("PERMS_ROLE_ID")
}

func loadDates() error {
	data, err := os.ReadFile(DatesFile)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &dates)
}

func addDate(date string) error {
	dates = append(dates, date)
	return saveDates()
}

func saveDates() error {
	data, err := json.MarshalIndent(dates, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(DatesFile, data, 0644)
}

func datesIncludes(date time.Time) bool {
	needle := date.Format(DateFormat)

	return slices.Contains(dates, needle)
}

func sendEvaderMessage(s *discordgo.Session, m *discordgo.Member, createdAt time.Time) error {
	message := fmt.Sprintf(
		":warning: **SUSPICIOUS JOIN**, new user <@!%[1]s> (**%[2]s**, `%[1]s`):\n"+
			"Account creation date **<t:%[3]d:D>** matches that of known alternate accounts used by ban evaders; all dates defined in master list.\n"+
			"*(No action taken, awaiting manual review — take appropriate action if necessary.)*", m.User.ID, m.User.Username, createdAt.Unix()*1000)
	_, err := s.ChannelMessageSend(alertsChannelID, message)
	return err
}

func sendRecentMessage(s *discordgo.Session, m *discordgo.Member, createdAt time.Time) error {
	message := fmt.Sprintf(
		":new: **NEW ACCOUNT**, new user <@!%[1]s> (**%[2]s**, `%[1]s`):\n"+
			"Account creation date **<t:%[3]d:D>** (<t:%[3]d:R>) is less than **24 hours old**.", m.User.ID, m.User.Username, createdAt.Unix())
	_, err := s.ChannelMessageSend(recentJoinChannelID, message)
	return err
}

func onMemberAdd(s *discordgo.Session, m *discordgo.GuildMemberAdd) {
	log.Printf("user joined:\n%+v\n\n", m)
	now := time.Now()

	// Retrieve the time.Time from the user's snowflake
	timeCreated, err := discordgo.SnowflakeTimestamp(m.User.ID)
	if err != nil {
		slog.Error("Could not parse the created time of the user account ID [%s]:\n%+v\n", m.User.ID, err)
	}

	// Get the difference between now and when the account was created (time.Duration)
	between := now.Sub(timeCreated)
	if between.Hours() < 24 {
		if err := sendRecentMessage(s, m.Member, timeCreated); err != nil {
			log.Print("failed to send message", err)
		}
	} else if datesIncludes(timeCreated) {
		if err := sendEvaderMessage(s, m.Member, timeCreated); err != nil {
			log.Print("failed to send message", err)
		}
	}
}

func hasAllowedRole(m *discordgo.MessageCreate) bool {
	return slices.Contains(m.Member.Roles, allowedRole)
}

func handleAddDate(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if !hasAllowedRole(m) {
		return
	}

	if len(args) < 2 {
		s.ChannelMessageSend(m.ChannelID, "Please provide a date in the format YYYY-MM-DD")
		return
	}

	dateStr := args[1]
	_, err := time.Parse("2006-01-02", dateStr)
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "Invalid date format. Please use the format YYYY-MM-DD")
		return
	}

	addDate(dateStr)

	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Date %s added successfully", dateStr))
}

func handleRemoveDate(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if !hasAllowedRole(m) {
		return
	}

	if len(args) < 2 {
		s.ChannelMessageSend(m.ChannelID, "Please provide a date to remove")
		return
	}

	dateToRemove := args[1]
	found := false

	for i, date := range dates {
		if date == dateToRemove {
			dates = slices.Delete(dates, i, 1)
			found = true
			break
		}
	}

	if found {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("%s has been removed from the list of dates", dateToRemove))
	} else {
		s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Date %s not found", dateToRemove))
	}
}

func handleAddDatesBetween(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if !hasAllowedRole(m) {
		return
	}

	if len(args) != 3 {
		s.ChannelMessageSend(m.ChannelID, "Please provide a start and end date in the format YYYY-MM-DD YYYY-MM-DD")
		return
	}

	start, err := time.Parse("2006-01-02", args[1])
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "Invalid start date format. Please use the format YYYY-MM-DD")
		return
	}

	end, err := time.Parse("2006-01-02", args[2])
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "Invalid end date format. Please use the format YYYY-MM-DD")
		return
	}

	current := start
	for current.Before(end) || current.Equal(end) {
		dateStr := current.Format("2006-01-02")
		if !contains(dates, dateStr) {
			dates = append(dates, dateStr)
		}
		current = current.AddDate(0, 0, 1)
	}

	sort.Strings(dates)
	saveDates()

	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Successfully added dates between %s and %s", args[1], args[2]))
}

func handleRemoveDatesBetween(s *discordgo.Session, m *discordgo.MessageCreate, args []string) {
	if !hasAllowedRole(m) {
		return
	}

	if len(args) != 3 {
		s.ChannelMessageSend(m.ChannelID, "Please provide a start and end date in the format YYYY-MM-DD YYYY-MM-DD")
		return
	}

	start, err := time.Parse("2006-01-02", args[1])
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "Invalid start date format. Please use the format YYYY-MM-DD")
		return
	}

	end, err := time.Parse("2006-01-02", args[2])
	if err != nil {
		s.ChannelMessageSend(m.ChannelID, "Invalid end date format. Please use the format YYYY-MM-DD")
		return
	}

	current := start
	datesToRemove := make(map[string]bool)
	for current.Before(end) || current.Equal(end) {
		datesToRemove[current.Format("2006-01-02")] = true
		current = current.AddDate(0, 0, 1)
	}

	newDates := []string{}
	for _, date := range dates {
		if !datesToRemove[date] {
			newDates = append(newDates, date)
		}
	}

	dates = newDates
	saveDates()

	s.ChannelMessageSend(m.ChannelID, fmt.Sprintf("Successfully removed dates between %s and %s", args[1], args[2]))
}

func handleListDates(s *discordgo.Session, m *discordgo.MessageCreate) {
	if !hasAllowedRole(m) {
		return
	}

	sort.Strings(dates)

	var groups [][]time.Time
	var currentGroup []time.Time

	for _, dateStr := range dates {
		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}

		if len(currentGroup) == 0 {
			currentGroup = append(currentGroup, date)
		} else if date.Sub(currentGroup[len(currentGroup)-1]) <= 24*time.Hour {
			currentGroup = append(currentGroup, date)
		} else {
			groups = append(groups, currentGroup)
			currentGroup = []time.Time{date}
		}
	}

	if len(currentGroup) > 0 {
		groups = append(groups, currentGroup)
	}

	var response strings.Builder

	for _, group := range groups {
		if len(group) == 1 {
			response.WriteString(group[0].Format("2006-01-02") + "\n")
		} else {
			response.WriteString(fmt.Sprintf("%s ➜ %s\n",
				group[0].Format("2006-01-02"),
				group[len(group)-1].Format("2006-01-02")))
		}
	}

	if response.Len() == 0 {
		s.ChannelMessageSend(m.ChannelID, "No dates found")
	} else {
		s.ChannelMessageSend(m.ChannelID, response.String())
	}
}

func onMessage(s *discordgo.Session, m *discordgo.MessageCreate) {
	if m.Author.ID == s.State.User.ID {
		return
	}

	args := strings.Fields(m.Content)
	if len(args) == 0 {
		return
	}

	switch args[0] {
	case "~add_date":
		handleAddDate(s, m, args)
	case "~remove_date", "~delete_date":
		handleRemoveDate(s, m, args)
	case "~add_dates_between":
		handleAddDatesBetween(s, m, args)
	case "~remove_dates_between", "~delete_dates_between":
		handleRemoveDatesBetween(s, m, args)
	case "~dates":
		handleListDates(s, m)
	}
}

func contains(slice []string, item string) bool {
	return slices.Contains(slice, item)
}
