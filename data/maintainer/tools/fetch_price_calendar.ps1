# Extract items from db-shops.js and packs from db-packs.js,
# then build a 31-day price calendar (peak-price sell list).
$ErrorActionPreference = "Stop"

$shopsJs = Get-Content "C:\Users\ALKAID\AppData\Local\Temp\souseha_dbshops.js" -Raw
$packsJs = Get-Content "C:\Users\ALKAID\AppData\Local\Temp\souseha_dbpacks.js" -Raw

# --- 1. extract item array: const e=[...]; ---
$itemMatch = [regex]::Match($shopsJs, 'const\s+e=(\[.*?\]);\s*export')
if (-not $itemMatch.Success) { throw "item array not found" }
$itemJson = $itemMatch.Groups[1].Value
# JS object literals use unquoted keys; convert to JSON by wrapping keys.
# Simpler: parse via a small JS-like transform: add quotes to known keys.
$keys = @('itemId','item','item_en','item_ja','item_ko','bdx_id','minRate','maxRate','base','bdx_type','maxPrice','category','recommend','highPremiumDay','highPremiumRate','highPremiumshopId')
foreach ($k in $keys) {
    $itemJson = $itemJson -replace "\b$k`":", "`"$k`":"
}
# also handle item_CN (present in some? check)
$itemJson = $itemJson -replace '\bitem_CN":', '"item_CN":'
$items = $itemJson | ConvertFrom-Json
Write-Output "items loaded: $($items.Count)"

# --- 2. extract packs object: me={"pack-s01":{...},...} ---
$packMatch = [regex]::Match($packsJs, 'me=(\{.*?\})\s*$')
if (-not $packMatch.Success) {
    # try finding from packTitle first occurrence backwards to '{'
    $pt = $packsJs.IndexOf('packTitle')
    if ($pt -lt 0) { throw "packTitle not found" }
    # find enclosing object start
    $start = $packsJs.LastIndexOf('={', $pt)
    if ($start -lt 0) { throw "packs object start not found" }
    $start = $start + 1
    # find matching close brace
    $depth = 1; $i = $start + 1
    while ($i -lt $packsJs.Length -and $depth -gt 0) {
        $c = $packsJs[$i]
        if ($c -eq '{') { $depth++ }
        elseif ($c -eq '}') { $depth-- }
        $i++
    }
    $packJson = $packsJs.Substring($start, $i - $start)
} else {
    $packJson = $packMatch.Groups[1].Value
}
# quote keys
$packKeys = @('packId','packTitle','packTitle_CN','packTitle_en','packTitle_ja','packTitle_ko','packType','huntingGround','huntingGround_CN','huntingGround_en','huntingGround_ja','huntingGround_ko','blueDot','greenDot','fight','hunting','shopId','codeName')
foreach ($k in $packKeys) {
    $packJson = $packJson -replace "\b$k`":", "`"$k`":"
}
$packsObj = $packJson | ConvertFrom-Json
$packs = $packsObj.PSObject.Properties | ForEach-Object { $_.Value }
Write-Output "packs loaded: $($packs.Count)"

# --- 3. build shopId -> CN name map ---
$shopMap = @{}
foreach ($p in $packs) {
    if ($p.shopId -and -not $shopMap.ContainsKey([string]$p.shopId)) {
        $name = $p.packTitle_CN
        if (-not $name) { $name = $p.packTitle }
        $shopMap[[string]$p.shopId] = $name
    }
}
Write-Output "shopIds with names: $($shopMap.Count)"

# --- 4. group items by highPremiumDay, collect shop names ---
$days = @{}
$shopSet = New-Object System.Collections.Generic.HashSet[string]
foreach ($it in $items) {
    $d = [string]$it.highPremiumDay
    if (-not $days.ContainsKey($d)) { $days[$d] = New-Object System.Collections.ArrayList }
    $shopName = if ($shopMap.ContainsKey([string]$it.highPremiumshopId)) { $shopMap[[string]$it.highPremiumshopId] } else { "shop-$($it.highPremiumshopId)" }
    [void]$shopSet.Add($shopName)
    [void]$days[$d].Add([PSCustomObject]@{
        item = $it.item
        shop = $shopName
        item_en = $it.item_en
        category = $it.category
    })
}
Write-Output "days covered: $($days.Count) ($(($days.Keys | Sort-Object {[int]$_}) -join ','))"
Write-Output "distinct shops: $($shopSet.Count)"

# --- 5. build price_calendar.v1.json ---
$shopList = @($shopSet | Sort-Object)
$calendar = [ordered]@{
    schema_version = 1
    updated_at = (Get-Date).ToString("yyyy-MM-ddTHH:mm:ss+08:00")
    timezone = "Asia/Shanghai"
    source_note = "Generated from browndust2-db.souseha.com client-side market data. Each item reaches its peak price (100 + highPremiumRate percent) on highPremiumDay at highPremiumshopId, computed by the site's deterministic PRNG. Items 29-31 have no peak day because highPremiumDay ranges 1..28."
    source_url = "https://browndust2-db.souseha.com/cn/market-data"
    shops = $shopList
    days = [ordered]@{}
}
for ($d = 1; $d -le 31; $d++) {
    $key = [string]$d
    if ($days.ContainsKey($key)) {
        $calendar.days[$key] = @($days[$key] | ForEach-Object {
            [ordered]@{
                item = $_.item
                shop = $_.shop
                aliases = @($_.item_en)
                reserve = 0
            }
        })
    } else {
        $calendar.days[$key] = @()
    }
}

$outPath = "f:\MABd2-wt\traecode-workspace\resource\price_calendar.v1.json"
$calendar | ConvertTo-Json -Depth 6 | Out-File -FilePath $outPath -Encoding utf8
$size = (Get-Item $outPath).Length
Write-Output "calendar written: $outPath ($size bytes)"

# quick stats
$totalSell = 0
foreach ($k in $calendar.days.Keys) { $totalSell += $calendar.days[$k].Count }
Write-Output "total sell entries across 31 days: $totalSell"
$missing = (1..31 | Where-Object { $calendar.days[[string]$_].Count -eq 0 })
Write-Output "days with no peak items: $($missing -join ',')"
