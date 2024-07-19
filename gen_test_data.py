import psycopg2
from psycopg2.extras import execute_batch
import random
import uuid

# Load the datasets
with open('first_names.txt') as f:
    first_names = [line.strip() for line in f]

with open('second_names.txt') as f:
    last_names = [line.strip() for line in f]

# Connect to your PostgreSQL database
conn = psycopg2.connect(
 dbname="default",
 user="default",
 password="default",
 host="localhost",
 port="5432"
)
cur = conn.cursor()

# Insert 1,000,000 rows in batches
batch_size = 100000
total_rows = 1000000

for _ in range(total_rows // batch_size):
 batch = [(str(uuid.uuid4()),random.choice(first_names), random.choice(last_names), None,'','','','','') for _ in range(batch_size)]
 execute_batch(cur, "INSERT INTO public.users (id,first_name, second_name, birthdate, sex, biography, city, username, password)\
                VALUES (%s,%s, %s, %s, %s, %s, %s, %s, %s)", batch)
 conn.commit()

cur.close()
conn.close()