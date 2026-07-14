import { NextResponse } from 'next/server';
import { readDb, writeDb } from '@/lib/db';

export async function POST(request: Request) {
  try {
    const payload = await request.json();
    const provider = request.headers.get('x-ota-provider') || 'bookingcom';
    
    const updates = Array.isArray(payload) ? payload : (payload.updates || []);
    const db = await readDb();
    
    let newInventory = [...db.inventory];
    
    for (const update of updates) {
      const idx = newInventory.findIndex(i => 
        i.provider === provider && 
        i.room_type_id === update.room_type_id && 
        i.date === update.date
      );
      
      const newItem = {
        provider,
        room_type_id: update.room_type_id,
        date: update.date,
        available: update.available,
        stop_sell: update.stop_sell,
        min_stay: update.min_stay,
        max_stay: update.max_stay,
      };
      
      if (idx >= 0) {
        newInventory[idx] = newItem;
      } else {
        newInventory.push(newItem);
      }
    }
    
    db.inventory = newInventory;
    await writeDb(db);
    
    return NextResponse.json({ success: true, updated: updates.length });
  } catch (err) {
    console.error(err);
    return NextResponse.json({ success: false, error: String(err) }, { status: 500 });
  }
}
