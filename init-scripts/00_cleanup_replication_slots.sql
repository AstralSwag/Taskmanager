-- Очистка неактивных слотов репликации
DO $$
DECLARE
    slot_record RECORD;
BEGIN
    FOR slot_record IN 
        SELECT slot_name 
        FROM pg_replication_slots 
        WHERE slot_name LIKE 'site_sub%' 
        AND NOT active
    LOOP
        PERFORM pg_drop_replication_slot(slot_record.slot_name);
        RAISE NOTICE 'Dropped inactive replication slot: %', slot_record.slot_name;
    END LOOP;
END $$; 