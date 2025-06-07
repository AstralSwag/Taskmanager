-- Очистка неактивных слотов репликации
DO $$
DECLARE
    slot_record RECORD;
BEGIN
    FOR slot_record IN 
        SELECT slot_name 
        FROM pg_replication_slots 
        WHERE (slot_name LIKE 'site_sub%' OR slot_name LIKE 'pg_%_sync_%')
        AND NOT active
    LOOP
        BEGIN
            PERFORM pg_drop_replication_slot(slot_record.slot_name);
            RAISE NOTICE 'Dropped inactive replication slot: %', slot_record.slot_name;
        EXCEPTION WHEN OTHERS THEN
            RAISE NOTICE 'Failed to drop slot %: %', slot_record.slot_name, SQLERRM;
        END;
    END LOOP;
END $$; 