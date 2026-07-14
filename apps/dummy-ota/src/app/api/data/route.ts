import { NextResponse } from 'next/server';
import { readDb } from '@/lib/db';

export async function GET(request: Request) {
  try {
    const db = await readDb();
    return NextResponse.json(db);
  } catch (err) {
    console.error(err);
    return NextResponse.json({ success: false, error: String(err) }, { status: 500 });
  }
}
