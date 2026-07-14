import { NextResponse } from 'next/server';
import { readDb, writeDb, Reservation } from '@/lib/db';

export async function GET(request: Request) {
  try {
    const url = new URL(request.url);
    const provider = request.headers.get('x-ota-provider') || url.searchParams.get('provider') || 'bookingcom';
    const since = url.searchParams.get('since');
    
    const db = await readDb();
    
    let reservations = db.reservations.filter(r => r.provider === provider);
    
    if (since) {
      const sinceDate = new Date(since).getTime();
      reservations = reservations.filter(r => new Date(r.created_at).getTime() >= sinceDate);
    }
    
    return NextResponse.json({ reservations });
  } catch (err) {
    console.error(err);
    return NextResponse.json({ success: false, error: String(err) }, { status: 500 });
  }
}

export async function POST(request: Request) {
  try {
    const payload = await request.json();
    const db = await readDb();
    
    const newReservation: Reservation = {
      id: Math.random().toString(36).substring(7),
      provider: payload.provider || 'bookingcom',
      status: 'confirmed',
      check_in: payload.check_in,
      check_out: payload.check_out,
      guest_name: payload.guest_name,
      room_type_id: payload.room_type_id,
      total_price: payload.total_price || 0,
      currency: payload.currency || 'USD',
      created_at: new Date().toISOString(),
    };
    
    db.reservations.push(newReservation);
    await writeDb(db);
    
    return NextResponse.json({ success: true, reservation: newReservation });
  } catch (err) {
    console.error(err);
    return NextResponse.json({ success: false, error: String(err) }, { status: 500 });
  }
}
