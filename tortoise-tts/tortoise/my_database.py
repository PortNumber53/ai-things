import os
import json
import psycopg2
from psycopg2 import sql
import logging
from dotenv import load_dotenv  # Import load_dotenv function

# Load environment variables from .env file
load_dotenv()

# Set up logging
logger = logging.getLogger(__name__)

def get_database_connection():
    try:
        conn = psycopg2.connect(
            dbname=os.getenv('DB_DATABASE'),
            user=os.getenv('DB_USERNAME'),
            password=os.getenv('DB_PASSWORD'),
            host=os.getenv('DB_HOST'),
            port=os.getenv('DB_PORT')
        )
        return conn
    except psycopg2.Error as e:
        logger.error(f"Error connecting to PostgreSQL: {e}")
        return None

def update_postgres_meta(content_id, filename, sentence_id=None):
    conn = get_database_connection()
    if conn is None:
        return

    try:
        cur = conn.cursor()
        # Fetch existing meta JSON
        cur.execute(sql.SQL("SELECT meta FROM contents WHERE id = %s"), (content_id,))
        row = cur.fetchone()
        if row:
            existing_meta = row[0] or {}  # Handle None case
            # Check if filename already exists in meta
            if "filenames" in existing_meta and any(entry["filename"] == f'{filename}' for entry in existing_meta["filenames"]):
                logger.warning(f"Filename {filename} already exists in meta for content_id {content_id}")
                return
            # Update meta with new filename
            existing_meta.setdefault("filenames", []).append({"filename": f'{filename}', "sentence_id": sentence_id})
            # Update the contents table with the new meta
            cur.execute(sql.SQL("UPDATE contents SET meta = %s WHERE id = %s"),
                        (json.dumps(existing_meta), content_id))
            conn.commit()
            logger.info(f"Meta updated for content_id {content_id} with filename {filename}")
        else:
            logger.error(f"No row found for content_id {content_id}")
    except psycopg2.Error as e:
        logger.error(f"Error updating PostgreSQL meta: {e}")
    finally:
        if conn:
            conn.close()
