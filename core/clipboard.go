package core

/*
#cgo CFLAGS: -x objective-c
#cgo LDFLAGS: -framework Cocoa
#import <Cocoa/Cocoa.h>
void writeRichTextToPasteboard(const char *text, const char *linkText, const char *url) {
    NSPasteboard *pasteboard = [NSPasteboard generalPasteboard];
    [pasteboard clearContents];

    // Create attributed string with the link
    NSString *fullText = [NSString stringWithFormat:@"%s %s", text, linkText];
    NSMutableAttributedString *attrString = [[NSMutableAttributedString alloc] initWithString:fullText];

    // Find the range of the link text
    NSString *linkTextNS = [NSString stringWithUTF8String:linkText];
    NSRange linkRange = [fullText rangeOfString:linkTextNS];

    if (linkRange.location != NSNotFound) {
        // Add link attribute
        NSURL *linkURL = [NSURL URLWithString:[NSString stringWithUTF8String:url]];
        [attrString addAttribute:NSLinkAttributeName value:linkURL range:linkRange];

        // Optional: make the link blue and underlined
        [attrString addAttribute:NSForegroundColorAttributeName
                          value:[NSColor blueColor]
                          range:linkRange];
        [attrString addAttribute:NSUnderlineStyleAttributeName
                          value:@(NSUnderlineStyleSingle)
                          range:linkRange];
    }

    // Write RTF data to pasteboard
    NSData *rtfData = [attrString RTFFromRange:NSMakeRange(0, [attrString length])
                            documentAttributes:@{}];
    [pasteboard setData:rtfData forType:NSPasteboardTypeRTF];

    // Also write plain text as fallback
    [pasteboard setString:fullText forType:NSPasteboardTypeString];
}
*/
import "C"
import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/atotto/clipboard"
)

// CopyRichText copies text with an embedded hyperlink to the macOS clipboard
// text: the plain text before the link
// linkText: the visible text of the link
// address: the URL (should include https://)
func copyRichTextMacOS(text, linkText, address string) {
	cText := C.CString(text)
	cLinkText := C.CString(linkText)
	cAddress := C.CString(address)

	defer C.free(unsafe.Pointer(cText))
	defer C.free(unsafe.Pointer(cLinkText))
	defer C.free(unsafe.Pointer(cAddress))

	C.writeRichTextToPasteboard(cText, cLinkText, cAddress)
}

func ClipboardWrite(pr *LocalPr, title string) {
	if runtime.GOOS == "darwin" {
		copyRichTextMacOS(":pr:", title, pr.Url())
	} else {
		clipboard.WriteAll(fmt.Sprintf(":pr: <%s|%s>", title, pr.Url()))
	}
}
