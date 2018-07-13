import Cocoa

extension URL {
    public var queryItems: [String: String] {
        var params = [String: String]()
        return URLComponents(url: self, resolvingAgainstBaseURL: false)?
            .queryItems?
            .reduce([:], { (_, item) -> [String: String] in
                params[item.name] = item.value
                return params
            }) ?? [:]
    }
}

var completePath = "";
for arg in CommandLine.arguments {
	completePath = arg;
}

let fileURL = URL(fileURLWithPath: completePath)
let fileManager = FileManager.default

var currentDate: NSDate? = nil
let sharingUrl = try! fileManager.url(forPublishingUbiquitousItemAt: fileURL, expiration: &currentDate)

let formatter = DateFormatter()
let myString = (String(describing: currentDate))
formatter.dateFormat = "yyyy-MM-dd HH:mm:ss"

let urlParams = sharingUrl.queryItems

var formattedUrl = urlParams["u"]!
formattedUrl = formattedUrl.replacingOccurrences(of: "${f}", with: urlParams["f"]!)
formattedUrl = formattedUrl.replacingOccurrences(of: "${uk}", with: urlParams["uk"]!)
print(formattedUrl)
