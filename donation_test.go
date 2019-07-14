package charityhonor

import (
	"fmt"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jmoiron/sqlx"
	"github.com/vindexus/ch-api/pkg/justgiving"
)

var numTestDrives = 0

func getDriveForTesting() (*Drive, *sqlx.Tx, *sqlx.DB) {
	numTestDrives++
	db := GetTestDb()
	drive := &Drive{
		SourceUrl: "http://www.reddit.com/r/number" + strconv.Itoa(numTestDrives),
	}

	tx, err := db.Beginx()
	if err != nil {
		panic(err)
	}
	if err = drive.Insert(tx); err != nil {
		panic(err)
	}

	return drive, tx, db
}

func TestDonationCRUD(t *testing.T) {
	drive, tx, db := getDriveForTesting()
	donation := Donation{
		DriveId:      drive.Id,
		CharityId:    1,
		Amount:       float64(12.34),
		CurrencyCode: "USD",
		DonorName:    "Vindexus",
		Message:      `I'm just trying this <strong>OUT!</strong>`,
	}

	if err := donation.Create(tx); err != nil {
		t.Error(err)
	}

	if donation.ReferenceCode == "" {
		t.Fatal("No reference code was created")
	}

	if donation.DriveId != drive.Id {
		t.Error("Drive Id doesn't match")
	}

	tx.Commit()

	tx, err := db.Beginx()
	dono2, err := GetDonationByReferenceCode(tx, donation.ReferenceCode)
	if err != nil {
		fmt.Println("ERR!")
		t.Error(err)
	}

	if dono2.Amount != donation.Amount {
		t.Errorf("Expected Amount '%v' but got '%v'", donation.Amount, dono2.Amount)
	}

	if dono2.CurrencyCode != donation.CurrencyCode {
		t.Errorf("Expected CurrencyCode '%v' but got '%v'", donation.CurrencyCode, dono2.CurrencyCode)
	}

	if dono2.Message != donation.Message {
		t.Errorf("Expected Message '%v' but got '%v'", donation.Message, dono2.Message)
	}

	if dono2.Charity == nil {
		t.Fatal("Donation's Charity property was nil")
	}

	if dono2 == nil || dono2.ReferenceCode != donation.ReferenceCode {
		t.Error("Ref code not made")
	}

	jg := justgiving.GetTestJG()

	url := dono2.GetDonationLink(jg)
	fmt.Println("url", url)

	if !strings.Contains(url, strconv.Itoa(justgiving.Fixtures.CharityId)) {
		t.Errorf("Url should contain %v, got %s", justgiving.Fixtures.CharityId, url)
	}

	tx.Commit()

	tx, _ = db.Beginx()
	newAmount := float64(1337)
	newName := "Colin 9430843290"
	dono2.Amount = newAmount
	dono2.DonorName = newName
	err = dono2.Save(tx)
	if err != nil {
		t.Fatal(err)
	}
	tx.Commit()

	dono3, err := GetDonationByReferenceCode(db, dono2.ReferenceCode)
	if err != nil {
		t.Fatal(err)
	}
	if dono3.DonorName != newName {
		t.Errorf("Expected name %v got %v", newName, dono3.DonorName)
	}

	if dono3.Amount != newAmount {
		t.Errorf("Expected amount %v got %v", newAmount, dono3.Amount)
	}
}

func TestDonationChecking(t *testing.T) {
	db := GetTestDb()
	tx, err := db.Beginx()
	if err != nil {
		t.Fatal(err)
	}
	dono, err := GetDonationById(tx, 1)
	if err != nil {
		t.Fatal(err)
	}

	jg := justgiving.GetTestJG()

	//Change the data from whatever's in the db to this.
	dono.ReferenceCode = justgiving.Fixtures.DonationReferenceCode
	dono.Status = DonationPending

	err = dono.CheckStatus(tx, jg)
	if err != nil {
		t.Fatal(err)
	}

	if dono.Status != DonationAccepted {
		t.Errorf("Expectd donation approved but was %v", dono.Status)
	}

	if dono.GetLastChecked().IsZero() {
		t.Error("last checked is zero")
	}

	if dono.GetLastChecked().Before(time.Now().Add(time.Second * -1)) {
		t.Error("Last checked should be older than 1s ago")
	}

	if dono.GetLastChecked().After(time.Now().Add(time.Second)) {
		t.Error("Last checked shouldn't be in the future")
	}

	dono.ReferenceCode = "nonexistantcode"
	dono.Status = DonationPending

	err = dono.CheckStatus(tx, jg)
	if err != nil {
		t.Fatal(err)
	}

	if dono.Status != DonationRejected {
		t.Error("Donation should be rejected if we can't find it in JG by reference code")
	}
}
