package main

import (
	"context"
	"fmt"
	wordsearcher "github.com/jwjones2/wordsearcher-server/wspb"
	"google.golang.org/grpc"
	"log"
	"time"
)

func main() {
	fmt.Println("Starting the client!")

	cc, err := grpc.Dial("localhost:50051", grpc.WithInsecure())
	if err != nil {
		log.Fatalf("Failed to connect to the server: %v", err)
	}
	defer func(cc *grpc.ClientConn) {
		err := cc.Close()
		if err != nil {
			log.Fatalf("Client close failed: %v", err)
		}
	}(cc)

	c := wordsearcher.NewWordsearcherServiceClient(cc)

	//doVerseUnary(c)
	//doVerseErrorCall(c)
	//doChapterCall(c)
	//doBiblePlanCall(c)
	//doBiblePlanDayCall(c)
	doSearch(c)
}

func doVerseUnary(c wordsearcher.WordsearcherServiceClient) {
	fmt.Println("Starting to do a Verse gRPC...")

	req := &wordsearcher.VerseRequest{
		Book:       1,
		Chapter:    1,
		VerseStart: 1,
		VerseEnd:   2,
	}
	res, err := c.Verse(context.Background(), req)
	if err != nil {
		log.Fatalf("Response failed: %v", err)
	}
	fmt.Printf("Response from server: Number of verse %v; Response: %v", len(res.Verses), res.Verses[1].GetText())
}

func doVerseErrorCall(c wordsearcher.WordsearcherServiceClient) {
	fmt.Println("Starting to do a Verse ERROR CALL...")

	req := &wordsearcher.VerseRequest{
		Book:       1,
		Chapter:    1,
		VerseStart: -3,
		VerseEnd:   2,
	}
	res, err := c.Verse(context.Background(), req)
	if err != nil {
		log.Fatalf("Response failed: %v", err)
	}
	fmt.Printf("Response from server: Number of verse %v; Response: %v", len(res.Verses), res.Verses[1].GetText())
}

func doChapterCall(c wordsearcher.WordsearcherServiceClient) {
	fmt.Println("Starting to do a Verse gRPC...")

	req := &wordsearcher.VerseRequest{
		Book:       1,
		Chapter:    1,
		VerseStart: 0,
		VerseEnd:   2,
	}
	res, err := c.Verse(context.Background(), req)
	if err != nil {
		log.Fatalf("Response failed: %v", err)
	}
	fmt.Printf("Response from server: Number of verse %v; Response: %v", len(res.Verses), res.Verses[1].GetText())
}

func doBiblePlanCall(c wordsearcher.WordsearcherServiceClient) {
	fmt.Println("Starting to do a Bible Plan gRPC...")

	req := &wordsearcher.BiblePlanRequest{
		Name: "McCheyneBasedYearly",
	}

	res, err := c.BiblePlan(context.Background(), req)
	if err != nil {
		log.Fatalf("Response failed: %v", err)
	}
	// get the day of the year to show the day
	t := time.Now()
	cDay := t.YearDay()
	fmt.Printf("Current day of the year is %d", cDay)
	fmt.Printf("Response from server: Number of verse %v; Response: %v", len(res.BiblePlan), res.BiblePlan[1].GetDays()[cDay-1])
}

func doBiblePlanDayCall(c wordsearcher.WordsearcherServiceClient) {
	fmt.Println("Starting to do a Bible Plan Day gRPC...")

	t := time.Now()
	cDay := t.YearDay() - 1
	req := &wordsearcher.BiblePlanDayRequest{
		Name: "McCheyneBasedYearly",
		Day:  int32(cDay),
	}

	res, err := c.BiblePlanDay(context.Background(), req)
	if err != nil {
		log.Fatalf("Response failed: %v", err)
	}
	fmt.Printf("Response from server: Response: %v", res.GetDay().GetReading2())
}

func doSearch(c wordsearcher.WordsearcherServiceClient) {
	fmt.Println("Starting to do a Search gRPC...")

	req := &wordsearcher.SearchRequest{
		Term:     "spake unto Joshua",
		Filter:   "exact",
		Location: "ot",
		Options:  "",
	}

	res, err := c.Search(context.Background(), req)
	if err != nil {
		log.Fatalf("Response failed: %v", err)
	}
	if len(res.GetVerses()) == 0 {
		fmt.Println("No verses found.")
	} else {
		fmt.Printf("Response from server: Number of Verses Found: %d; Response: %v", len(res.GetVerses()), res.GetVerses()[0])
	}
}
