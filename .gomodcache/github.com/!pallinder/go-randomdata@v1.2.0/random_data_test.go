package randomdata

import (
	"bytes"
	"math/rand"
	"net"
	"reflect"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"golang.org/x/text/language"
)

func TestCustomRand(t *testing.T) {
	t.Log("TestCustomRand")
	r1 := rand.New(rand.NewSource(1))
	r2 := rand.New(rand.NewSource(1))

	CustomRand(r1)
	s1 := RandStringRunes(10)

	CustomRand(r2)
	s2 := RandStringRunes(10)

	if s1 != s2 {
		t.Fatal("Strings should have matched")
	}
}

func TestTitle(t *testing.T) {
	t.Log("TestTitle")
	titleMale := Title(Male)
	titleFemale := Title(Female)
	randomTitle := Title(100)

	if !findInSlice(jsonData.MaleTitles, titleMale) {
		t.Error("titleMale empty or not in male titles")
	}

	if !findInSlice(jsonData.FemaleTitles, titleFemale) {
		t.Error("firstNameFemale empty or not in female titles")
	}

	names := make([]string, len(jsonData.MaleTitles)+len(jsonData.FemaleTitles))
	names = append(names, jsonData.MaleTitles...)
	names = append(names, jsonData.FemaleTitles...)
	if !findInSlice(names, randomTitle) {
		t.Error("randomName empty or not in male and female titles")
	}
}

func TestRandomStringDigits(t *testing.T) {
	t.Log("TestRandomStringDigits")

	if len(StringNumber(2, "-")) != 5 {
		t.Fatal("Wrong length returned")
	}

	if len(StringNumber(2, "")) != 4 {
		t.Fatal("Wrong length returned")
	}

	if len(StringNumberExt(3, "/", 3)) != 11 {
		t.Fatal("Wrong length returned")
	}

	if len(StringNumberExt(3, "", 3)) != 9 {
		t.Fatal("Wrong length returned")
	}
}

func TestFirstName(t *testing.T) {
	t.Log("TestFirstName")
	firstNameMale := FirstName(Male)
	firstNameFemale := FirstName(Female)
	randomName := FirstName(RandomGender)

	if !findInSlice(jsonData.FirstNamesMale, firstNameMale) {
		t.Error("firstNameMale empty or not in male names")
	}

	if !findInSlice(jsonData.FirstNamesFemale, firstNameFemale) {
		t.Error("firstNameFemale empty or not in female names")
	}

	if randomName == "" {
		t.Error("randomName empty")
	}

}

func TestLastName(t *testing.T) {
	t.Log("TestLastName")
	lastName := LastName()

	if !findInSlice(jsonData.LastNames, lastName) {
		t.Error("lastName empty or not in slice")
	}
}

func TestFullName(t *testing.T) {
	t.Log("TestFullName")

	fullNameMale := FullName(Male)
	fullNameFemale := FullName(Female)
	fullNameRandom := FullName(RandomGender)

	maleSplit := strings.Fields(fullNameMale)
	femaleSplit := strings.Fields(fullNameFemale)
	randomSplit := strings.Fields(fullNameRandom)

	if len(maleSplit) == 0 {
		t.Error("Failed on full name male")
	}

	if !findInSlice(jsonData.FirstNamesMale, maleSplit[0]) {
		t.Error("Couldnt find maleSplit first name in firstNamesMale")
	}

	if !findInSlice(jsonData.LastNames, maleSplit[1]) {
		t.Error("Couldnt find maleSplit last name in lastNames")
	}

	if len(femaleSplit) == 0 {
		t.Error("Failed on full name female")
	}

	if !findInSlice(jsonData.FirstNamesFemale, femaleSplit[0]) {
		t.Error("Couldnt find femaleSplit first name in firstNamesFemale")
	}

	if !findInSlice(jsonData.LastNames, femaleSplit[1]) {
		t.Error("Couldnt find femaleSplit last name in lastNames")
	}

	if len(randomSplit) == 0 {
		t.Error("Failed on full name random")
	}

	if !findInSlice(jsonData.FirstNamesMale, randomSplit[0]) && !findInSlice(jsonData.FirstNamesFemale, randomSplit[0]) {
		t.Error("Couldnt find randomSplit first name in either firstNamesMale or firstNamesFemale")
	}

}

func TestEmail(t *testing.T) {
	t.Log("TestEmail")
	email := Email()

	if email == "" {
		t.Error("Failed to generate email with content")
	}

}

func TestCountry(t *testing.T) {
	t.Log("TestCountry")
	countryFull := Country(FullCountry)
	countryTwo := Country(TwoCharCountry)
	countryThree := Country(ThreeCharCountry)

	if len(countryThree) < 3 {
		t.Error("countryThree < 3 chars")
	}

	if !findInSlice(jsonData.Countries, countryFull) {
		t.Error("Couldnt find country in countries")
	}

	if !findInSlice(jsonData.CountriesTwoChars, countryTwo) {
		t.Error("Couldnt find country with two chars in countriesTwoChars")
	}

	if !findInSlice(jsonData.CountriesThreeChars, countryThree) {
		t.Error("Couldnt find country with three chars in countriesThreeChars")
	}
}

func TestCurrency(t *testing.T) {
	t.Log("TestCurrency")
	if !findInSlice(jsonData.Currencies, Currency()) {
		t.Error("Could not find currency in currencies")
	}
}

func TestCity(t *testing.T) {
	t.Log("TestCity")
	city := City()

	if !findInSlice(jsonData.Cities, city) {
		t.Error("Couldnt find city in cities")
	}
}

func TestParagraph(t *testing.T) {
	t.Log("TestParagraph")
	paragraph := Paragraph()

	if !findInSlice(jsonData.Paragraphs, paragraph) {
		t.Error("Couldnt find paragraph in paragraphs")
	}
}

func TestAlphanumeric(t *testing.T) {
	t.Log("TestAlphanumeric")
	alphanumric := Alphanumeric(10)
	if len(alphanumric) != 10 {
		t.Error("alphanumric has wrong size")
	}
	re := regexp.MustCompile(`^[[:alnum:]]+$`)
	if !re.MatchString(alphanumric) {
		t.Error("alphanumric contains invalid character")
	}
}

func TestBool(t *testing.T) {
	t.Log("TestBool")
	booleanVal := Boolean()
	if booleanVal != true && booleanVal != false {
		t.Error("Bool was wrong format")
	}
}

func TestState(t *testing.T) {
	t.Log("TestState")
	stateValSmall := State(Small)
	stateValLarge := State(Large)

	if !findInSlice(jsonData.StatesSmall, stateValSmall) {
		t.Error("Couldnt find small state name in states")
	}

	if !findInSlice(jsonData.States, stateValLarge) {
		t.Error("Couldnt find state name in states")
	}

}

func TestNoun(t *testing.T) {
	if len(jsonData.Nouns) == 0 {
		t.Error("Nouns is empty")
	}

	noun := Noun()

	if !findInSlice(jsonData.Nouns, noun) {
		t.Error("Couldnt find noun in json data")
	}
}

func TestAdjective(t *testing.T) {
	if len(jsonData.Adjectives) == 0 {
		t.Error("Adjectives array is empty")
	}

	adjective := Adjective()

	if !findInSlice(jsonData.Adjectives, adjective) {
		t.Error("Couldnt find noun in json data")
	}
}

func TestSillyName(t *testing.T) {
	sillyName := SillyName()

	if len(sillyName) == 0 {
		t.Error("Couldnt generate a silly name")
	}
}

func TestIpV4Address(t *testing.T) {
	ipAddress := IpV4Address()

	ipBlocks := strings.Split(ipAddress, ".")

	if len(ipBlocks) < 0 || len(ipBlocks) > 4 {
		t.Error("Invalid generated IP address")
	}

	for _, blockString := range ipBlocks {
		blockNumber, err := strconv.Atoi(blockString)

		if err != nil {
			t.Error("Error while testing IpV4Address(): " + err.Error())
		}

		if blockNumber < 0 || blockNumber > 255 {
			t.Error("Invalid generated IP address")
		}
	}
}

func TestIpV6Address(t *testing.T) {
	ipAddress := net.ParseIP(IpV6Address())

	if len(ipAddress) != net.IPv6len {
		t.Errorf("Invalid generated IPv6 address %v", ipAddress)
	}
	roundTripIP := net.ParseIP(ipAddress.String())
	if roundTripIP == nil || !bytes.Equal(ipAddress, roundTripIP) {
		t.Errorf("Invalid generated IPv6 address %v", ipAddress)
	}
}

func TestMacAddress(t *testing.T) {
	t.Log("MacAddress")

	mac := MacAddress()
	if len(mac) != 17 {
		t.Errorf("Invalid generated Mac address %v", mac)
	}

	if !regexp.MustCompile(`([0-9a-fa-f]{2}[:-]){5}([0-9a-fa-f]{2})`).MatchString(mac) {
		t.Errorf("Invalid generated Mac address %v", mac)
	}
}

func TestDecimal(t *testing.T) {
	d := Decimal(2, 4, 3)
	if !(d >= 2 && d <= 4) {
		t.Error("Invalid generate range")
	}

	ds := strings.Split(strconv.FormatFloat(d, 'f', 3, 64), ".")
	if len(ds[1]) != 3 {
		t.Error("Invalid floating point")
	}
}

func TestDay(t *testing.T) {
	t.Log("TestDay")
	day := Day()

	if !findInSlice(jsonData.Days, day) {
		t.Error("Couldnt find day in days")
	}
}

func TestMonth(t *testing.T) {
	t.Log("TestMonth")
	month := Month()

	if !findInSlice(jsonData.Months, month) {
		t.Error("Couldnt find month in months")
	}
}

func TestStringSample(t *testing.T) {
	t.Log("TestStringSample")
	list := []string{"str1", "str2", "str3"}
	str := StringSample(list...)
	if reflect.TypeOf(str).String() != "string" {
		t.Error("Didn't get a string object")
	}
	if !findInSlice(list, str) {
		t.Error("Didn't get string from sample list")
	}
}

func TestStringSampleEmptyList(t *testing.T) {
	t.Log("TestStringSample")
	str := StringSample()
	if reflect.TypeOf(str).String() != "string" {
		t.Error("Didn't get a string object")
	}
	if str != "" {
		t.Error("Didn't get empty string for empty sample list")
	}
}

func TestFullDate(t *testing.T) {
	t.Log("TestFullDate")
	fulldateOne := FullDate()
	fulldateTwo := FullDate()

	_, err := time.Parse(DateOutputLayout, fulldateOne)
	if err != nil {
		t.Error("Invalid random full date")
	}

	_, err = time.Parse(DateOutputLayout, fulldateTwo)
	if err != nil {
		t.Error("Invalid random full date")
	}

	if fulldateOne == fulldateTwo {
		t.Error("Generated same full date twice in a row")
	}
}

func TestFullDatePenetration(t *testing.T) {
	for i := 0; i < 100000; i += 1 {
		d := FullDate()
		_, err := time.Parse(DateOutputLayout, d)
		if err != nil {
			t.Error("Invalid random full date")
		}
	}
}
func TestFullDateInRangeNoArgs(t *testing.T) {
	t.Log("TestFullDateInRangeNoArgs")
	fullDate := FullDateInRange()
	_, err := time.Parse(DateOutputLayout, fullDate)

	if err != nil {
		t.Error("Didn't get valid date format.")
	}
}

func TestFullDateInRangeOneArg(t *testing.T) {
	t.Log("TestFullDateInRangeOneArg")
	maxDate, _ := time.Parse(DateInputLayout, "2016-12-31")
	for i := 0; i < 10000; i++ {
		fullDate := FullDateInRange("2016-12-31")
		d, err := time.Parse(DateOutputLayout, fullDate)

		if err != nil {
			t.Error("Didn't get valid date format.")
		}

		if d.After(maxDate) {
			t.Error("Random date didn't match specified max date.")
		}
	}
}

func TestFullDateInRangeTwoArgs(t *testing.T) {
	t.Log("TestFullDateInRangeTwoArgs")
	minDate, _ := time.Parse(DateInputLayout, "2016-01-01")
	maxDate, _ := time.Parse(DateInputLayout, "2016-12-31")
	for i := 0; i < 10000; i++ {
		fullDate := FullDateInRange("2016-01-01", "2016-12-31")
		d, err := time.Parse(DateOutputLayout, fullDate)

		if err != nil {
			t.Error("Didn't get valid date format.")
		}

		if d.After(maxDate) {
			t.Error("Random date didn't match specified max date.")
		}

		if d.Before(minDate) {
			t.Error("Random date didn't match specified min date.")
		}
	}
}

func TestFullDateInRangeSwappedArgs(t *testing.T) {
	t.Log("TestFullDateInRangeSwappedArgs")
	wrongMaxDate, _ := time.Parse(DateInputLayout, "2016-01-01")
	fullDate := FullDateInRange("2016-12-31", "2016-01-01")
	d, err := time.Parse(DateOutputLayout, fullDate)

	if err != nil {
		t.Error("Didn't get valid date format.")
	}

	if d != wrongMaxDate {
		t.Error("Didn't return min date.")
	}
}

func TestTimezone(t *testing.T) {
	t.Log("TestTimezone")
	timezone := Timezone()

	if !findInSlice(jsonData.Timezones, timezone) {
		t.Errorf("Couldnt find timezone in timezones: %v", timezone)
	}
}

func TestLocale(t *testing.T) {
	t.Log("TestLocale")
	locale := Locale()
	_, err := language.Parse(locale)
	if err != nil {
		t.Errorf("Invalid locale: %v", locale)
	}
}

func TestLocalePenetration(t *testing.T) {
	t.Log("TestLocale")

	for i := 0; i < 10000; i += 1 {
		locale := Locale()
		_, err := language.Parse(locale)
		if err != nil {
			t.Errorf("Invalid locale: %v", locale)
		}
	}
}

func TestUserAgentString(t *testing.T) {
	t.Log("UserAgentString")

	ua := UserAgentString()
	if len(ua) == 0 {
		t.Error("Empty User Agent String")
	}

	if !regexp.MustCompile(`^[a-zA-Z]+\/[0-9]+.[0-9]+\ \(.*\).*$`).MatchString(ua) {
		t.Errorf("Invalid generated User Agent String: %v", ua)
	}
}

func findInSlice(source []string, toFind string) bool {
	for _, text := range source {
		if text == toFind {
			return true
		}
	}
	return false
}

func TestPhoneNumbers(t *testing.T) {
	CheckPhoneNumber(PhoneNumber(), t)
}

func CheckPhoneNumber(str string, t *testing.T) bool {
	if (len(str) - strings.Count(str, " ")) > 16 {
		t.Error("phone number too long")
		return false
	}

	matched, err := regexp.MatchString("\\+\\d{1,3}\\s\\d{1,3}", str)

	if err != nil {
		t.Errorf("error matching %v", err)
		return false
	}

	if !matched {
		t.Error("phone number did not match expectations")
		return false
	}

	return true
}

func TestProvinceForCountry(t *testing.T) {
	supportedCountries := []string{"US", "GB"}
	for _, c := range supportedCountries {
		p := ProvinceForCountry(c)
		if p == "" {
			t.Errorf("did not return a valid province for country %s", c)
		}
		switch c {
		case "US":
			if !findInSlice(jsonData.States, p) {
				t.Errorf("did not return a known province for US")
			}
		case "GB":
			if !findInSlice(jsonData.ProvincesGB, p) {
				t.Errorf("did not return a known province for GB")
			}
		}
	}

	p := ProvinceForCountry("bogus")
	if p != "" {
		t.Errorf("did not return empty province for unknown country")
	}
}

func TestStreetForCountry(t *testing.T) {
	supportedCountries := []string{"US", "GB"}
	for _, c := range supportedCountries {
		p := StreetForCountry(c)
		if p == "" {
			t.Errorf("did not return a valid street for country %s", c)
		}
	}

	p := StreetForCountry("bogus")
	if p != "" {
		t.Errorf("did not return empty street for unknown country")
	}
}
