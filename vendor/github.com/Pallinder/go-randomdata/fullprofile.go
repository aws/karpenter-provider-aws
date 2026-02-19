package randomdata

import (
	"crypto/md5"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"strconv"
	"strings"
)

var letterRunes = []rune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ")
var portraitDirs = []string{"men", "women"}

type Profile struct {
	Gender string `json:"gender"`
	Name   struct {
		First string `json:"first"`
		Last  string `json:"last"`
		Title string `json:"title"`
	} `json:"name"`
	Location struct {
		Street   string `json:"street"`
		City     string `json:"city"`
		State    string `json:"state"`
		Postcode int    `json:"postcode"`
	} `json:"location"`

	Email string `json:"email"`
	Login struct {
		Username string `json:"username"`
		Password string `json:"password"`
		Salt     string `json:"salt"`
		Md5      string `json:"md5"`
		Sha1     string `json:"sha1"`
		Sha256   string `json:"sha256"`
	} `json:"login"`

	Dob        string `json:"dob"`
	Registered string `json:"registered"`
	Phone      string `json:"phone"`
	Cell       string `json:"cell"`

	ID struct {
		Name  string      `json:"name"`
		Value interface{} `json:"value"`
	} `json:"id"`

	Picture struct {
		Large     string `json:"large"`
		Medium    string `json:"medium"`
		Thumbnail string `json:"thumbnail"`
	} `json:"picture"`
	Nat string `json:"nat"`
}

func RandStringRunes(n int) string {
	b := make([]rune, n)
	for i := range b {
		b[i] = letterRunes[privateRand.Intn(len(letterRunes))]
	}
	return string(b)
}

func getMD5Hash(text string) string {
	hasher := md5.New()
	hasher.Write([]byte(text))
	return hex.EncodeToString(hasher.Sum(nil))
}

func getSha1(text string) string {
	hasher := sha1.New()
	hasher.Write([]byte(text))
	sha := base64.URLEncoding.EncodeToString(hasher.Sum(nil))
	return sha
}

func getSha256(text string) string {
	hasher := sha256.New()
	hasher.Write([]byte(text))
	sha := base64.URLEncoding.EncodeToString(hasher.Sum(nil))
	return sha
}

func GenerateProfile(gender int) *Profile {
	profile := &Profile{}
	if gender == Male {
		profile.Gender = "male"
	} else if gender == Female {
		profile.Gender = "female"
	} else {
		gender = privateRand.Intn(2)
		if gender == Male {
			profile.Gender = "male"
		} else {
			profile.Gender = "female"
		}
	}
	profile.Name.Title = Title(gender)
	profile.Name.First = FirstName(gender)
	profile.Name.Last = LastName()
	profile.ID.Name = "SSN"
	profile.ID.Value = fmt.Sprintf("%d-%d-%d",
		Number(101, 999),
		Number(01, 99),
		Number(100, 9999),
	)

	profile.Email = strings.ToLower(profile.Name.First) + "." + strings.ToLower(profile.Name.Last) + "@example.com"
	profile.Cell = PhoneNumber()
	profile.Phone = PhoneNumber()
	profile.Dob = FullDate()
	profile.Registered = FullDate()
	profile.Nat = "US"

	profile.Location.City = City()
	i, _ := strconv.Atoi(PostalCode("US"))
	profile.Location.Postcode = i
	profile.Location.State = State(2)
	profile.Location.Street = StringNumber(1, "") + " " + Street()

	profile.Login.Username = SillyName()
	pass := SillyName()
	salt := RandStringRunes(16)
	profile.Login.Password = pass
	profile.Login.Salt = salt
	profile.Login.Md5 = getMD5Hash(pass + salt)
	profile.Login.Sha1 = getSha1(pass + salt)
	profile.Login.Sha256 = getSha256(pass + salt)

	pic := privateRand.Intn(35)
	profile.Picture.Large = fmt.Sprintf("https://randomuser.me/api/portraits/%s/%d.jpg", portraitDirs[gender], pic)
	profile.Picture.Medium = fmt.Sprintf("https://randomuser.me/api/portraits/med/%s/%d.jpg", portraitDirs[gender], pic)
	profile.Picture.Thumbnail = fmt.Sprintf("https://randomuser.me/api/portraits/thumb/%s/%d.jpg", portraitDirs[gender], pic)

	return profile
}
