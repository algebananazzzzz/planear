package input_test

import (
	"testing"

	"github.com/algebananazzzzz/planear/pkg/input"
	"github.com/algebananazzzzz/planear/testutils"
	"github.com/stretchr/testify/require"
)

type SimpleRecord struct {
	ID   string `csv:"id"`
	Name string `csv:"name"`
	Age  int    `csv:"age"`
}

type PtrRecord struct {
	ID    string  `csv:"id"`
	Email *string `csv:"email"`
	Score *int    `csv:"score"`
}

type NoTagRecord struct {
	Id   string
	Name string
	Age  int
}

type UnsupportedFieldRecord struct {
	ID    string  `csv:"id"`
	Extra float64 `csv:"extra"`
}

func TestDecodeCSVFile_SimpleRecords(t *testing.T) {
	dir := testutils.NewTestDir(t)
	content := []byte("id,name,age\n1,Alice,30\n2,Bob,25\n")
	path := testutils.CreateMockFile(t, dir, "simple.csv", content)

	records, err := input.DecodeCSVFile[SimpleRecord](path)
	require.NoError(t, err)
	require.Len(t, records, 2)
	require.Equal(t, "Alice", records[0].Name)
	require.Equal(t, 30, records[0].Age)
	require.Equal(t, "Bob", records[1].Name)
	require.Equal(t, 25, records[1].Age)
}

func TestDecodeCSVFile_PointerFields(t *testing.T) {
	dir := testutils.NewTestDir(t)
	content := []byte("id,email,score\n1,alice@example.com,100\n2,,\n")
	path := testutils.CreateMockFile(t, dir, "ptr.csv", content)

	records, err := input.DecodeCSVFile[PtrRecord](path)
	require.NoError(t, err)
	require.Len(t, records, 2)

	require.NotNil(t, records[0].Email)
	require.Equal(t, "alice@example.com", *records[0].Email)
	require.NotNil(t, records[0].Score)
	require.Equal(t, 100, *records[0].Score)

	require.Nil(t, records[1].Email)
	require.Nil(t, records[1].Score)
}

func TestDecodeCSVFile_MissingColumns_Error(t *testing.T) {
	dir := testutils.NewTestDir(t)
	content := []byte("id,name\n1,Alice\n2,Bob\n") // missing "age" column
	path := testutils.CreateMockFile(t, dir, "missing_cols.csv", content)

	_, err := input.DecodeCSVFile[SimpleRecord](path)
	require.Error(t, err)
	require.ErrorContains(t, err, "missing required column")
}

func TestDecodeCSVFile_ExtraColumnsAllowed(t *testing.T) {
	dir := testutils.NewTestDir(t)
	content := []byte("id,name,age,extra\n1,Alice,30,something\n2,Bob,25,another\n")
	path := testutils.CreateMockFile(t, dir, "extra_cols.csv", content)

	records, err := input.DecodeCSVFile[SimpleRecord](path)
	require.NoError(t, err)
	require.Len(t, records, 2)
}

func TestDecodeCSVFile_EmptyFile_Error(t *testing.T) {
	dir := testutils.NewTestDir(t)
	content := []byte("")
	path := testutils.CreateMockFile(t, dir, "empty.csv", content)

	_, err := input.DecodeCSVFile[SimpleRecord](path)
	require.Error(t, err)
	require.ErrorContains(t, err, "empty CSV file")
}

func TestDecodeCSVFile_OnlyHeader_NoRows(t *testing.T) {
	dir := testutils.NewTestDir(t)
	content := []byte("id,name,age\n")
	path := testutils.CreateMockFile(t, dir, "only_header.csv", content)

	records, err := input.DecodeCSVFile[SimpleRecord](path)
	require.NoError(t, err)
	require.Len(t, records, 0)
}

func TestDecodeCSVFile_InvalidIntValue_Error(t *testing.T) {
	dir := testutils.NewTestDir(t)
	content := []byte("id,name,age\n1,Alice,notanint\n")
	path := testutils.CreateMockFile(t, dir, "invalid_int.csv", content)

	_, err := input.DecodeCSVFile[SimpleRecord](path)
	require.Error(t, err)
	require.ErrorContains(t, err, "invalid int value")
}

func TestDecodeCSVFile_UnsupportedFieldType_Error(t *testing.T) {
	dir := testutils.NewTestDir(t)
	content := []byte("id,extra\n1,3.14\n")
	path := testutils.CreateMockFile(t, dir, "unsupported_type.csv", content)

	_, err := input.DecodeCSVFile[UnsupportedFieldRecord](path)
	require.Error(t, err)
	require.ErrorContains(t, err, "unsupported field type")
}

func TestDecodeCSVFile_NoCSVTags_AllFieldsEmpty(t *testing.T) {
	// When a struct has no csv tags, DecodeCSVFile won't populate any fields
	// because fieldMap will be empty. This is by design - csv tags are required.
	dir := testutils.NewTestDir(t)
	content := []byte("id,name,age\n1,Alice,30\n2,Bob,25\n")
	path := testutils.CreateMockFile(t, dir, "notags.csv", content)

	records, err := input.DecodeCSVFile[NoTagRecord](path)
	require.NoError(t, err)
	require.Len(t, records, 2)
	// Fields are empty because no csv tags defined the mapping
	require.Equal(t, "", records[0].Name)
	require.Equal(t, 0, records[0].Age)
}

func TestDecodeCSVFile_PointerStructType_Error(t *testing.T) {
	dir := testutils.NewTestDir(t)
	content := []byte("id,email,score\n1,alice@example.com,100\n2,,\n")
	path := testutils.CreateMockFile(t, dir, "ptr_struct.csv", content)

	_, err := input.DecodeCSVFile[*PtrRecord](path)
	require.Error(t, err)
	require.ErrorContains(t, err, "type parameter T must be a struct")
}

func TestDecodeCSVFile_NonStructType_Error(t *testing.T) {
	dir := testutils.NewTestDir(t)
	// create a dummy CSV file; content doesn’t matter since type check is before reading
	content := []byte("id,name\n1,Alice\n")
	path := testutils.CreateMockFile(t, dir, "dummy.csv", content)

	_, err := input.DecodeCSVFile[int](path) // int is not a struct, should error
	require.Error(t, err)
	require.ErrorContains(t, err, "type parameter T must be a struct")
}

func TestDecodeCSVFile_CannotSetField_Error(t *testing.T) {
	dir := testutils.NewTestDir(t)
	// CSV header includes an unexported field name "unexported"
	content := []byte("exported,unexported\nhello,world\n")
	path := testutils.CreateMockFile(t, dir, "unsettable.csv", content)

	type Unsettable struct {
		Exported   string `csv:"exported"`
		unexported string `csv:"unexported"` // unexported, not settable by reflection
	}

	_, err := input.DecodeCSVFile[Unsettable](path)
	require.Error(t, err)
	require.ErrorContains(t, err, "cannot set field 'unexported'")
}

func TestDecodeCSVFile_InvalidIntPointerValue(t *testing.T) {
	type Rec struct {
		ID    string `csv:"id"`
		Score *int   `csv:"score"`
	}

	dir := testutils.NewTestDir(t)
	// Provide invalid int in score column ("abc")
	content := []byte("id,score\n1,abc\n")
	path := testutils.CreateMockFile(t, dir, "invalid_int.csv", content)

	_, err := input.DecodeCSVFile[Rec](path)
	require.Error(t, err)
	require.ErrorContains(t, err, "invalid int value for field 'score' at row 2")
}

func TestDecodeCSVFile_UnsupportedPointerElementType_Error(t *testing.T) {
	dir := testutils.NewTestDir(t)
	content := []byte("unsupported_field\nsomevalue\n")
	path := testutils.CreateMockFile(t, dir, "unsupported_pointer.csv", content)

	type UnsupportedPointer struct {
		UnsupportedField *float64 `csv:"unsupported_field"` // float64 pointer unsupported
	}

	_, err := input.DecodeCSVFile[UnsupportedPointer](path)
	require.Error(t, err)
	require.ErrorContains(t, err, "unsupported pointer element type for field 'unsupported_field'")
}

func TestDecodeCSVFile_IntPointerEmptyValue_SetsNil(t *testing.T) {
	type Rec struct {
		ID    string `csv:"id"`
		Score *int   `csv:"score"`
	}

	dir := testutils.NewTestDir(t)
	content := []byte("id,score\n1,\n2,5\n")
	path := testutils.CreateMockFile(t, dir, "empty_int_ptr.csv", content)

	records, err := input.DecodeCSVFile[Rec](path)
	require.NoError(t, err)
	require.Len(t, records, 2)

	// record 1 score should be nil because empty string in CSV
	require.Nil(t, records[0].Score)

	// record 2 score should be pointer with value 5
	require.NotNil(t, records[1].Score)
	require.Equal(t, 5, *records[1].Score)
}

func TestDecodeCSVFile_IntFieldEmptyValue_SetsZero(t *testing.T) {
	type Rec struct {
		ID    string `csv:"id"`
		Score int    `csv:"score"`
	}

	dir := testutils.NewTestDir(t)
	content := []byte("id,score\n1,\n2,7\n")
	path := testutils.CreateMockFile(t, dir, "empty_int.csv", content)

	records, err := input.DecodeCSVFile[Rec](path)
	require.NoError(t, err)
	require.Len(t, records, 2)

	// record 1 score should be 0 because empty string in CSV
	require.Equal(t, 0, records[0].Score)

	// record 2 score should be 7
	require.Equal(t, 7, records[1].Score)
}
