import * as SQLite from 'expo-sqlite';
import { ADDED_COLUMNS, ALL_SCHEMA, SCHEMA_VERSION } from './schema';

const DATABASE_NAME = 'logistics.db';

let db: SQLite.SQLiteDatabase | null = null;

export async function getDatabase(): Promise<SQLite.SQLiteDatabase> {
  if (!db) {
    db = await SQLite.openDatabaseAsync(DATABASE_NAME);
    await initializeDatabase(db);
  }
  return db;
}

export async function initializeDatabase(database: SQLite.SQLiteDatabase): Promise<void> {
  await database.execAsync('PRAGMA journal_mode = WAL;');

  // Creates anything missing. Existing tables are left alone, which is why
  // migrateSchema below has to add columns separately.
  for (const schema of ALL_SCHEMA) {
    await database.execAsync(schema);
  }

  await migrateSchema(database);
}

/**
 * Brings an existing database up to SCHEMA_VERSION.
 *
 * Installs created before version tracking report user_version 0 even though
 * they already have the v1 tables, so a missing column — not the recorded
 * version — decides whether a step runs. Each step is therefore idempotent, and
 * a fresh database created from ALL_SCHEMA passes straight through.
 */
async function migrateSchema(database: SQLite.SQLiteDatabase): Promise<void> {
  const { user_version: currentVersion } =
    (await database.getFirstAsync<{ user_version: number }>('PRAGMA user_version')) ?? {
      user_version: 0,
    };

  if (currentVersion >= SCHEMA_VERSION) {
    return;
  }

  for (let version = currentVersion + 1; version <= SCHEMA_VERSION; version++) {
    for (const { table, column, definition } of ADDED_COLUMNS[version] ?? []) {
      await addColumnIfMissing(database, table, column, definition);
    }
  }

  // PRAGMA does not accept bound parameters.
  await database.execAsync(`PRAGMA user_version = ${SCHEMA_VERSION}`);
}

async function addColumnIfMissing(
  database: SQLite.SQLiteDatabase,
  table: string,
  column: string,
  definition: string
): Promise<void> {
  const columns = await database.getAllAsync<{ name: string }>(`PRAGMA table_info(${table})`);
  if (columns.some((existing) => existing.name === column)) {
    return;
  }

  await database.execAsync(`ALTER TABLE ${table} ADD COLUMN ${column} ${definition}`);
}

export async function closeDatabase(): Promise<void> {
  if (db) {
    await db.closeAsync();
    db = null;
  }
}

export async function resetDatabase(): Promise<void> {
  const database = await getDatabase();

  await database.execAsync(`
    DROP TABLE IF EXISTS shipments;
    DROP TABLE IF EXISTS tracking_events;
    DROP TABLE IF EXISTS proof_of_delivery;
    DROP TABLE IF EXISTS status_updates;
    DROP TABLE IF EXISTS sync_queue;
    DROP TABLE IF EXISTS dead_letter_queue;
    DROP TABLE IF EXISTS sync_metadata;
    PRAGMA user_version = 0;
  `);

  await initializeDatabase(database);
}

export { DATABASE_NAME };
