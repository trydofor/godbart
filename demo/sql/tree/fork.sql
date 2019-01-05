-- VAR now 'TIME-NOW'
SELECT NOW() as now;

-- REF id 'A.id'
SELECT id FROM A;

-- REF ib 'B.id'
SELECT id FROM B where aid = 'A.id' and upd < 'TIME-NOW'

-- REF ib 'C.id'
SELECT id FROM C where bid = 'B.id' and upd < 'TIME-NOW'

-- RUN FOR 'A.id'  调整分叉
SELECT id FROM D where bid = 'B.id' and aid = 'A.id'


-- REF eib 'E.id'
-- REF fib 'F.id'  联合多个REF，避免多分叉
SELECT E.id as eid, F.id as fid FROM E,F limit 3

SELECT id FROM G where eid = 'E.id' and fid='F.id'
