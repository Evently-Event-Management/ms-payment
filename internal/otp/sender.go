package otp

import (
	"fmt"
	"log"
	"net/smtp"
)

func SendEmailOTP(toEmail, otp string) {
	from := "isurumuniwije@gmail.com" // e.g., yourcompany@gmail.com
	password := "yotp eehv mcnq osnh" // App password for Gmail
	smtpHost := "smtp.gmail.com"
	smtpPort := "587"

	if from == "" || password == "" {
		log.Fatal("SMTP configuration environment variables are missing")
	}
	// HTML Styled Message
	message := []byte(fmt.Sprintf(
		"Subject: 🎟 Your Eventify OTP Code\r\n"+
			"MIME-version: 1.0;\r\n"+
			"Content-Type: text/html; charset=\"UTF-8\";\r\n\r\n"+
			`<div style="font-family: Arial, sans-serif; max-width: 500px; margin: auto; border: 2px dashed #FF6600; border-radius: 10px; padding: 20px; background-color: #fff9f2;">
				<div style="text-align: center;">
					<img src="https://yourcdn.com/eventify-logo.png" alt="Eventify" style="max-width: 120px; margin-bottom: 15px;">
					<h2 style="color: #FF6600;">🎟 Eventify Ticket OTP</h2>
					<p style="font-size: 16px; color: #555;">Use the following OTP to verify your ticket purchase:</p>
					<div style="font-size: 32px; font-weight: bold; color: #000; background-color: #FFE0CC; padding: 10px; display: inline-block; border-radius: 8px; letter-spacing: 4px;">
						%s
					</div>
					<p style="font-size: 14px; color: #888; margin-top: 15px;">
						This OTP will expire in 5 minutes. Please do not share it with anyone.
					</p>
				</div>
			</div>`, otp))

	// Authentication
	auth := smtp.PlainAuth("", from, password, smtpHost)

	// Send Email
	err := smtp.SendMail(smtpHost+":"+smtpPort, auth, from, []string{toEmail}, message)
	if err != nil {
		log.Fatal("Failed to send email:", err)
	}
	fmt.Println("✅ OTP sent to", toEmail)
}
