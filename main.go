package main

import (
	"fmt"
	"math/rand"
	"os"
	"strconv"

	gin "gopkg.in/gin-gonic/gin.v1"

	"bytes"
	"log"
	"net/http"

	"io"
	"time"

	"database/sql"

	"mime/multipart"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/awsutil"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/kidsdynamic/childrenlab_avatar/database"
	"github.com/urfave/cli"
)

type Kid struct {
	ID              int64     `json:"id"`
	Name            string    `json:"name"`
	DateCreated     time.Time `json:"dateCreated" db:"date_created"`
	MacID           string    `json:"macId" db:"mac_id"`
	Profile         *string   `json:"profile"`
	FirmwareVersion *string   `json:"-" db:"firmware_version"`
	ParentID        int64     `json:"-" db:"parent_id"`
	Parent          User      `json:"parent"`
}

type User struct {
	ID                       int64     `json:"id" db:"id"`
	Password                 string    `json:"-"`
	Email                    string    `json:"email" db:"email"`
	FirstName                string    `json:"firstName" db:"first_name"`
	LastName                 string    `json:"lastName" db:"last_name"`
	LastUpdated              time.Time `json:"lastUpdate" db:"last_updated"`
	DateCreated              time.Time `json:"dateCreated" db:"date_created"`
	ZipCode                  string    `json:"zipCode" db:"zip_code"`
	PhoneNumber              string    `json:"phoneNumber" db:"phone_number"`
	Profile                  string    `json:"profile"`
	Language                 string    `json:"language"`
	RegistrationID           string    `json:"-" db:"registration_id"`
	AndroidRegistrationToken string    `json:"-" db:"android_registration_token"`
	Role                     Role      `json:"-"`
	RoleID                   int64     `json:"-" db:"role_id"`
	ResetPasswordToken       string    `json:"-" db:"reset_password_token"`
	SignUpIP                 string    `json:"-" db:"sign_up_ip"`
	SignUpCountryCode        string    `json:"country" db:"sign_up_country_code"`
}

type Role struct {
	ID        int64  `json:"-"`
	Authority string `json:"authority"`
}

type AwsConfiguration struct {
	Bucket    string
	Region    string
	AccessKey string
}

type FwFile struct {
	ID           int64
	Version      string
	FileAURL     string
	FIleBURL     string
	UploadedDate time.Time
}

var awsConfig AwsConfiguration

func main() {
	app := cli.NewApp()
	app.Name = "childrenlab"

	app.Flags = []cli.Flag{
		cli.StringFlag{
			EnvVar: "DEBUG",
			Name:   "debug",
			Usage:  "Debug",
			Value:  "false",
		},
		cli.StringFlag{
			EnvVar: "DATABASE_USER",
			Name:   "database_user",
			Usage:  "Database user name",
			Value:  "",
		},
		cli.StringFlag{
			EnvVar: "DATABASE_PASSWORD",
			Name:   "database_password",
			Usage:  "Database password",
			Value:  "",
		},
		cli.StringFlag{
			EnvVar: "DATABASE_IP",
			Name:   "database_IP",
			Usage:  "Database IP address with port number",
			Value:  "",
		},
		cli.StringFlag{
			EnvVar: "DATABASE_NAME",
			Name:   "database_name",
			Usage:  "Database name",
			Value:  "swing_test_record",
		},
		cli.StringFlag{
			EnvVar: "AWS_BUCKET",
			Name:   "aws_bucket",
			Usage:  "bucket name",
			Value:  "",
		},
		cli.StringFlag{
			EnvVar: "AWS_REGION",
			Name:   "aws_region",
			Usage:  "AWS region",
			Value:  "",
		},
		cli.StringFlag{
			EnvVar: "AWS_ACCESS_KEY_ID",
			Name:   "aws_access_key",
			Usage:  "bucket name",
			Value:  "",
		},
		cli.StringFlag{
			EnvVar: "AWS_SECRET_ACCESS_KEY",
			Name:   "aws_secret_acess_key",
			Usage:  "bucket name",
			Value:  "",
		},
	}

	app.Action = func(c *cli.Context) error {
		database.Database = database.DatabaseInfo{
			Name:     c.String("database_name"),
			User:     c.String("database_user"),
			Password: c.String("database_password"),
			IP:       c.String("database_IP"),
		}
		//fmt.Println(c.String("aws_bucket"))
		c.Set("aws_bucket", c.String("aws_bucket"))

		awsConfig = AwsConfiguration{
			Bucket:    c.String("aws_bucket"),
			Region:    c.String("aws_region"),
			AccessKey: c.String("aws_access_key"),
		}

		fmt.Printf("Database: %v", database.Database)

		r := gin.Default()

		if c.Bool("debug") == true {
			gin.SetMode(gin.DebugMode)
		} else {
			gin.SetMode(gin.ReleaseMode)
		}

		r.Use(gin.Logger())
		r.Use(gin.Recovery())

		r.POST("/v1/user/avatar/upload", UploadAvatar)
		r.POST("/v1/user/avatar/uploadKid", UploadKidAvatar)
		r.POST("/v1/admin/fwFile", UploadFWFile)

		return r.Run(":8112")
		/*
			if c.Bool("debug") {
			} else {
				return r.RunTLS(":8112", ".ssh/childrenlab.chained.crt", ".ssh/childrenlab.com.key")
			}*/

	}

	app.Run(os.Args)
}

func getUserID(c *gin.Context) int64 {
	authToken := c.Request.Header.Get("x-auth-token")
	if authToken == "" {
		return 0
	}

	db := database.NewDatabase()
	defer db.Close()

	var userID int64

	err := db.Get(&userID, "SELECT u.id FROM user u JOIN authentication_token a ON u.email = a.email WHERE token = ?", authToken)
	if err != nil && userID != 0 {
		return 0
	}

	return userID

}
func UploadAvatar(c *gin.Context) {
	userID := getUserID(c)
	if userID == 0 {
		c.JSON(http.StatusForbidden, gin.H{})
		c.Abort()
		return
	}
	file, _, err := c.Request.FormFile("upload")
	if err != nil {
		log.Printf("Error on save avatar: %#v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "upload parameter is required",
			"error":   err,
		})
		return
	}

	db := database.NewDatabase()
	defer db.Close()
	var user User
	if err := db.Get(&user, "SELECT id, email, first_name, last_name, last_updated, date_created, zip_code, phone_number, profile, language FROM user WHERE id = ? LIMIT 1", userID); err != nil {
		fmt.Printf("%#v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Something wrong when getting user",
			"error":   err,
		})
		return
	}
	oldFileName := user.Profile
	fileName := oldFileName

	for fileName == oldFileName {
		fileName = fmt.Sprintf("avatar_%d-%d.jpg", userID, randomNumber())
	}

	if os.MkdirAll("./tmp", 0755) != nil {
		panic("Unable to create directory for tagfile!")
	}

	out, err := os.OpenFile("./tmp/"+fileName, os.O_CREATE|os.O_RDWR, 0755)
	if err != nil {
		log.Println(err)
	}
	defer out.Close()
	_, err = io.Copy(out, file)
	if err != nil {
		log.Println(err)
	}

	f, err := os.Open("./tmp/" + fileName)
	if err != nil {
		log.Println(err)
	}

	if err = UploadFileToS3(f, fmt.Sprintf("/userProfile/%s", fileName)); err == nil {
		if _, err := db.Exec("UPDATE user SET profile = ? WHERE id = ?", fileName, userID); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Something wrong when updating profile for the user",
				"error":   err,
			})
			return
		}

		user.Profile = fileName

		c.JSON(http.StatusOK, gin.H{
			"user": user,
		})

		if len(oldFileName) > 0 {
			if err := removeFileFromS3(fmt.Sprintf("/userProfile/%s", oldFileName)); err != nil {
				if aerr, ok := err.(awserr.Error); ok {
					switch aerr.Code() {
					default:
						fmt.Println(aerr.Error())
					}
				} else {
					// Print the error, cast err to awserr.Error to get the Code and
					// Message from an error.
					fmt.Println(err.Error())
				}
				return
			}
		}
	} else {
		fmt.Printf("Error on upload user image to S3. Error: %#v\n", err)
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error on upload image to S3",
			"error":   err,
		})
	}
}

func UploadKidAvatar(c *gin.Context) {

	userID := getUserID(c)
	file, _, err := c.Request.FormFile("upload")

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "upload parameter is required",
			"error":   err,
		})
		return
	}

	db := database.NewDatabase()
	defer db.Close()

	kidID, err := strconv.ParseInt(c.PostForm("kidId"), 10, 64)

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Error on parse KidId",
			"error":   err,
		})
		return
	}

	var kid Kid
	if err = db.Get(&kid, "SELECT * FROM kids WHERE parent_id = ? AND id = ?", userID, kidID); err != nil {
		log.Printf("Error on get kid from database. Error: %#v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Error on Get kid from database",
			"error":   err,
		})
		return
	}

	var user User
	if err = db.Get(&user, "SELECT * FROM user WHERE id = ?", kid.ParentID); err != nil {
		log.Printf("Error on get kid from database. Error: %#v", err)
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Error on Get kid from database",
			"error":   err,
		})
		return
	}
	kid.Parent = user

	oldFileName := *kid.Profile
	fileName := oldFileName

	for fileName == oldFileName {
		fileName = fmt.Sprintf("kid_avatar_%d-%d.jpg", kidID, randomNumber())
	}
	if err != nil {
		fmt.Printf("err opening file: %s", err)
	}

	if os.MkdirAll("./tmp", 0755) != nil {

		panic("Unable to create directory for tagfile!")

	}

	out, err := os.OpenFile("./tmp/"+fileName, os.O_CREATE|os.O_RDWR, 0755)
	if err != nil {
		log.Println(err)
	}
	defer out.Close()
	_, err = io.Copy(out, file)
	if err != nil {
		log.Println(err)
	}

	f, err := os.Open("./tmp/" + fileName)
	if err != nil {
		log.Println(err)
	}

	if UploadFileToS3(f, fmt.Sprintf("/userProfile/%s", fileName)) == nil {

		_, err := db.Exec("UPDATE kids SET profile = ? WHERE id = ?", fileName, kidID)

		if err != nil {
			log.Printf("Error on update profile. Error: %#v", err)
		}

		oldFilename := kid.Profile

		kid.Profile = &fileName

		c.JSON(http.StatusOK, gin.H{
			"kid": kid,
		})

		if len(*oldFilename) > 0 {
			if err := removeFileFromS3(fmt.Sprintf("/userProfile/%s", *oldFilename)); err != nil {
				if aerr, ok := err.(awserr.Error); ok {
					switch aerr.Code() {
					default:
						fmt.Println(aerr.Error())
					}
				} else {
					// Print the error, cast err to awserr.Error to get the Code and
					// Message from an error.
					fmt.Println(err.Error())
				}
				return
			}
		}

	}

}

func UploadFWFile(c *gin.Context) {
	fileA, _, err := c.Request.FormFile("fileA")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "File A parameter is required",
			"error":   err,
		})
		return
	}

	fileB, _, err := c.Request.FormFile("fileB")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "File B parameter is required",
			"error":   err,
		})
		return
	}

	if os.MkdirAll("./tmp", 0755) != nil {
		panic("Unable to create directory for tagfile!")
	}

	versionName := c.Request.FormValue("versionName")
	if versionName == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Version name is required",
			"error":   err,
		})
		return
	}

	db := database.NewDatabase()
	defer db.Close()

	var fwFile FwFile

	if err := db.Get(&fwFile, "SELECT * FROM fw_file WHERE version_name = ?", versionName); err != nil {
		if err != sql.ErrNoRows {
			fmt.Println(err)
		}
	}

	if fwFile.ID != 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"message": "Version name exists",
			"error":   err,
		})
		return
	}

	localFileA, localFileAPath, err := getFilePath(versionName, "A", fileA)
	if err != nil {
		log.Panic(err)
	}

	localFileB, localFileBPath, err := getFilePath(versionName, "B", fileB)
	if err != nil {
		log.Panic(err)
	}

	if err := UploadFileToS3(localFileA, localFileAPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error on uploading file to S3",
			"error":   err,
		})
		return
	}

	if err := UploadFileToS3(localFileB, localFileBPath); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"message": "Error on uploading file to S3",
			"error":   err,
		})
		return
	}

	if _, err := db.Exec("INSERT INTO fw_file (version, file_a_url, file_b_url, uploaded_date) VALUES (?, ?, ?, NOW())", versionName, localFileAPath, localFileBPath); err != nil {
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"message": "Error on inserting database",
				"error":   err,
			})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{})

}

func getFilePath(version, name string, file multipart.File) (*os.File, string, error) {
	fileName := fmt.Sprintf("./tmp/%s%s.%s", version, name, "hex")
	out, err := os.OpenFile(fileName, os.O_CREATE|os.O_RDWR, 0755)
	if err != nil {
		return nil, "", err
	}
	defer out.Close()
	_, err = io.Copy(out, file)
	if err != nil {
		return nil, "", err
	}

	f, err := os.Open(fileName)
	if err != nil {
		return nil, "", err
	}

	filePath := fmt.Sprintf("fw_version/%s%s.hex", version, name)

	return f, filePath, nil
}

func UploadFileToS3(file *os.File, filePath string) error {

	ss, err := session.NewSession()
	if err != nil {
		log.Fatal(err)
	}
	_, err = ss.Config.Credentials.Get()
	if err != nil {
		log.Fatal(err)
	}
	svc := s3.New(session.New(&aws.Config{}))

	fileInfo, _ := file.Stat()
	var size int64 = fileInfo.Size()

	buffer := make([]byte, size)
	file.Read(buffer)
	fileBytes := bytes.NewReader(buffer)
	fileType := http.DetectContentType(buffer)

	uploadResult, err := svc.PutObject(&s3.PutObjectInput{
		Body:          fileBytes,
		Bucket:        aws.String(awsConfig.Bucket),
		Key:           aws.String(filePath),
		ContentLength: aws.Int64(size),
		ContentType:   aws.String(fileType),
		ACL:           aws.String("public-read"),
	})
	if err != nil {
		log.Printf("Failed to upload data to %s\n", err)
		return err
	}

	log.Printf("Response: %s\n", awsutil.StringValue(uploadResult))

	return nil

}

func removeFileFromS3(fileName string) error {
	ss, err := session.NewSession()
	if err != nil {
		log.Fatal(err)
	}
	_, err = ss.Config.Credentials.Get()
	if err != nil {
		return err
	}
	svc := s3.New(session.New(&aws.Config{}))
	input := &s3.DeleteObjectInput{
		Bucket: aws.String(awsConfig.Bucket),
		Key:    aws.String(fileName),
	}

	_, err = svc.DeleteObject(input)
	if err != nil {
		return err
	}

	return nil
}

func randomNumber() int {
	s1 := rand.NewSource(time.Now().UnixNano())
	r1 := rand.New(s1)

	return r1.Intn(9999)
}
