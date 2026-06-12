from sqlmodel import SQLModel, create_engine, Session
from sqlalchemy import event
import os

DATABASE_URL = os.getenv("DATABASE_URL", "./nightcode.db")
engine = create_engine(f"sqlite:///{DATABASE_URL}", echo=False)

@event.listens_for(engine, "connect")
def set_sqlite_pragma(dbapi_conn, _):
    cursor = dbapi_conn.cursor()
    cursor.execute("PRAGMA journal_mode=WAL")
    cursor.execute("PRAGMA synchronous=NORMAL")
    cursor.execute("PRAGMA foreign_keys=ON")
    cursor.close()

def create_tables():
    SQLModel.metadata.create_all(engine)

def get_session():
    with Session(engine) as session:
        yield session
