ALTER TABLE search_cycle_stats
  ADD CONSTRAINT fk_search_cycle_stats_search
  FOREIGN KEY (search_id) REFERENCES searches(id) ON DELETE CASCADE;
