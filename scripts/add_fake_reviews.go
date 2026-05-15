package main

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func main() {
	ctx := context.Background()

	// Connect to the database
	dsn := "postgresql://postgres.lawhapcawlvjbkdmnhir:5y%2542!.6zcP1@aws-1-ap-southeast-1.pooler.supabase.com:5432/postgres"
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		log.Fatalf("Unable to connect to database: %v\n", err)
	}
	defer pool.Close()

	// Test connection
	if err := pool.Ping(ctx); err != nil {
		log.Fatalf("Unable to ping database: %v\n", err)
	}
	fmt.Println("Connected to database!")

	// Sample movie IDs (TMDB) - 60 unique IDs
	movieIDs := []int{
		550, 238, 424, 155, 680, 13, 11, 603, 27205, 129,
		120, 429, 122, 278, 240, 637, 1891, 598, 12477, 335984,
		24428, 453395, 353081, 284053, 301528, 566, 453, 324857, 284052, 299537,
		363088, 315635, 324552, 271110, 299536, 320288, 290859, 346698, 299534, 353491,
		324856, 244786, 20662, 244785, 122917, 157336, 140607, 19404, 299535, 346697,
		346696, 10719, 12445, 6033, 280, 597, 98, 580, 105, 608,
	}

	// Review comments - varying lengths
	shortReviews := []string{
		"Great movie!",
		"Loved it!",
		"Amazing film!",
		"Highly recommend.",
		"Masterpiece!",
		"Brilliant!",
		"5 stars!",
		"Epic!",
		"Best movie ever!",
		"Watch it now!",
		"Terrible.",
		"Disappointing.",
		"Overrated.",
		"Could be better.",
		"Not my style.",
		"Boring.",
		"Just okay.",
		"Decent watch.",
		"Good but long.",
		"Fun movie!",
	}

	mediumReviews := []string{
		"Really enjoyed this film. The acting was top-notch and the story kept me engaged throughout. Would definitely watch again.",
		"A solid movie with great performances. The cinematography is beautiful and the soundtrack complements the scenes perfectly.",
		"Good movie overall but had some pacing issues in the middle. Still worth watching for the excellent performances.",
		"This film exceeded my expectations. The director did a fantastic job of bringing the story to life.",
		"Average movie with some memorable moments. Not groundbreaking but entertaining enough for a watch.",
		"Really liked the character development in this movie. Each character felt authentic and well-written.",
		"The plot was intriguing and kept me guessing. Some parts dragged a bit but the ending made up for it.",
		"Visually stunning film with a compelling story. The director's vision really shines through in every scene.",
		"Not bad at all. Had some laughs and some emotional moments. Would recommend to friends who enjoy this genre.",
		"Decent film with good chemistry between the leads. The screenplay could have been tighter though.",
		"Enjoyed it more than I expected. The action sequences were well choreographed and the story was engaging.",
		"Good vibes overall. The soundtrack was perfect for the mood. Might rewatch sometime.",
		"Hit and miss for me. Some parts were brilliant while others felt unnecessary.",
		"Really appreciate the attention to detail in this film. You can tell a lot of work went into it.",
		"Entertaining from start to finish. The pacing was good and it never felt dragged out.",
	}

	longReviews := []string{
		"I absolutely loved this movie! From the opening scene to the final credits, I was completely engaged. The performances were phenomenal, especially the lead actor who delivered an incredibly nuanced performance. The screenplay was tight and well-crafted, with dialogues that felt natural and meaningful. The cinematography was stunning - every frame could be a painting. The director really understood the source material and brought a unique vision to the screen. The soundtrack was perfectly selected and enhanced every emotional beat. While no film is perfect, this one comes pretty close. It's the kind of movie that stays with you long after watching it. I've already recommended it to several friends and will definitely be watching it again in the future. A must-watch for anyone who appreciates great cinema.",
		"This film was a mixed bag for me. On one hand, the production values were excellent - the set design, costume work, and visual effects were all top-notch. The acting was generally good, with some standout performances from the supporting cast. On the other hand, the story felt underdeveloped in places and some of the character motivations weren't clear. The pacing was uneven - the first act dragged a bit while the third act felt rushed. I appreciated what the director was trying to do thematically, but the execution didn't quite land for me. Still, there were enough good moments to make it worth watching. If you're a fan of the genre or the director's other work, you'll probably enjoy this. Otherwise, you might want to give it a pass unless you're very bored.",
		"Wow, what an experience! This movie really surprised me. I went in with modest expectations and came out completely blown away. The story was original and refreshing - so tired of the same old formulas! The characters were complex and relatable, and I found myself really caring about what happened to them. The tension was built beautifully, with moments of quiet intensity interspersed with some truly breathtaking set pieces. The writing was sharp and witty, with several lines that made me laugh out loud. The emotional moments hit hard without feeling manipulative. Special mention to the score - it was haunting and beautiful and perfectly matched the tone of the film. This is the kind of movie that reminds me why I love cinema. Can't wait to see it again!",
		"Let me start by saying that I'm not usually a fan of this genre, but this movie changed my mind. The director has a distinctive visual style that sets it apart from everything else out there. The world-building was impressive - I was immediately drawn into this universe and wanted to know more. The lore felt rich and layered without being overwhelming. The protagonist's journey was compelling and their growth throughout the film felt earned. There were moments of pure brilliance here - scenes that I'll remember for years to come. However, I did feel that the middle portion could have been tightened up a bit. Some subplots didn't get the resolution they deserved. Still, the positives far outweigh the negatives. Definitely worth your time.",
		"I'm torn on this one. The film has some genuinely great moments and some real problems. Starting with the good: the cast is excellent, with each actor bringing their A-game. The action choreography was inventive and exciting. The production design was gorgeous - clearly a lot of money and care went into making this look special. The score was epic and rousing. Now for the bad: the script has some serious issues. There are logical inconsistencies that bothered me throughout. Some characters were poorly defined and their actions didn't always make sense. The dialogue was hit or miss - some exchanges were sharp and witty, others felt clunky and expository. The runtime was too long and could have easily trimmed 20-30 minutes without losing anything important. Overall, it's an ambitious film that doesn't quite achieve what it sets out to do, but it's still entertaining enough to recommend with reservations.",
		"One of the best films of the year! The filmmakers clearly poured their hearts into this project and it shows in every frame. Every aspect of production - from the meticulous set design to the gorgeous cinematography to the powerful score - worked in harmony to create something truly special. The performances were universally excellent. The lead actor delivered a career-defining performance that will be remembered for years. The supporting cast was equally impressive, with each character leaving a lasting impression. The story was simple but effective, and the themes resonated with me long after the credits rolled. This is the kind of film that restores your faith in Hollywood. It's rare to find a blockbuster that manages to be both hugely entertaining and genuinely meaningful. Don't miss it!",
		"I've been looking forward to this movie for months and unfortunately it didn't quite live up to my expectations. That's not to say it's bad - it's actually quite good in many ways. The visual effects were spectacular and the action sequences were thrilling. The world is richly detailed and immersive. However, the story felt thin and overly familiar. It seemed like the filmmakers were so focused on the spectacle that they forgot to give us compelling characters to care about. The protagonist was more of a cipher than a person. The emotional beats fell flat because I didn't have any investment in the outcome. It's a fun popcorn movie but don't think too hard about it. Worth watching for the visuals alone if nothing else.",
		"This is a deeply personal and emotional film that left me feeling moved and thoughtful. The director has crafted something really special here - a story that manages to be both intimate and universal at the same time. The performances are subtle and nuanced, with every glance and gesture carrying meaning. The script is smart and doesn't treat its audience as stupid. It trusts you to pick up on subtext and draw your own conclusions. The pacing is deliberate and allows each scene to breathe. Some viewers might find it slow, but I appreciated the chance to savor each moment. The ending is perfect - neither too neat nor too ambiguous. It respects the audience's intelligence while still providing emotional satisfaction. This is cinema at its finest.",
	}

	// Insert 60 reviews
	rand.Seed(time.Now().UnixNano())
	
	for i := 0; i < 60; i++ {
		movieID := movieIDs[i%len(movieIDs)]
		rating := rand.Intn(5) + 1 // 1-5 rating

		// Pick random review based on length distribution
		var comment string
		roll := rand.Intn(100)
		switch {
		case roll < 30: // 30% short
			comment = shortReviews[rand.Intn(len(shortReviews))]
		case roll < 80: // 50% medium
			comment = mediumReviews[rand.Intn(len(mediumReviews))]
		default: // 20% long
			comment = longReviews[rand.Intn(len(longReviews))]
		}

		// Random date within the last 30 days
		daysAgo := rand.Intn(30)
		createdAt := time.Now().AddDate(0, 0, -daysAgo).Add(time.Duration(rand.Intn(24)) * time.Hour)

		_, err := pool.Exec(ctx, `
			INSERT INTO user_reviews (user_id, movie_id, rating, comment, created_at)
			VALUES ($1, $2, $3, $4, $5)
		`, 48, movieID, rating, comment, createdAt)

		if err != nil {
			log.Printf("Failed to insert review %d: %v\n", i+1, err)
		} else {
			fmt.Printf("Inserted review %d: movie=%d, rating=%d, len=%d chars\n", i+1, movieID, rating, len(comment))
		}

		// Small delay to spread out timestamps slightly
		time.Sleep(10 * time.Millisecond)
	}

	fmt.Println("\nDone! Inserted 60 fake reviews for user 48.")
}