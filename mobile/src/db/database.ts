import * as SQLite from 'expo-sqlite';
import { ALL_SCHEMA } from './schema';

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
  
  for (const schema of ALL_SCHEMA) {
    await database.execAsync(schema);
  }
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
    DROP TABLE IF EXISTS sync_queue;
    DROP TABLE IF EXISTS sync_metadata;
  `);
  
  await initializeDatabase(database);
}

export { DATABASE_NAME };
