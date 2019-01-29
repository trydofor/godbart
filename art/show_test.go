package art

import (
	"regexp"
	"testing"
)

func Test_Show(t *testing.T) {
	ktpl := []string{
`tbl`, `
DROP TABLE IF EXISTS ${TABLE_NAME};
${TABLE_DDL};
`,

`trg`, `
DROP TRIGGER IF EXISTS ${TRIGGER_NAME};
DELIMITER $$
${TRIGGER_DDL} $$
DELIMITER ;
`,

`log`, `
DROP TABLE IF EXISTS ${TABLE_NAME}$log ;
-- CREATE TABLE ${TABLE_NAME}$log AS SELECT * FROM ${TABLE_NAME} WHERE 1=0;

CREATE TABLE ${TABLE_NAME}$log (
  ${COLUMNS_FULL},
  _id int(11) NOT NULL AUTO_INCREMENT,
  _du int(11) DEFAULT NULL,
  _dt datetime DEFAULT NULL,
  PRIMARY KEY (_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

DROP TRIGGER IF EXISTS ${TABLE_NAME}$log$bu;
DELIMITER $$
CREATE TRIGGER ${TABLE_NAME}$log$bu BEFORE UPDATE ON ${TABLE_NAME}
FOR EACH ROW BEGIN
  insert into ${TABLE_NAME}$log select *, null, 1, now() from ${TABLE_NAME}
  where id= OLD.id ;
END $$
DELIMITER ;

DROP TRIGGER IF EXISTS ${TABLE_NAME}$log$bd;
DELIMITER $$
CREATE TRIGGER ${TABLE_NAME}$log$bd BEFORE DELETE ON ${TABLE_NAME}
FOR EACH ROW BEGIN
  insert into ${TABLE_NAME}$log select *, null, 2, now() from ${TABLE_NAME}
  where id= OLD.id ;
END $$
DELIMITER ;`,
	}
	rgx := []*regexp.Regexp{regexp.MustCompile("tx_parcel")}
	Show(dsrc, ktpl, rgx)
}
