import unittest
from computer_use import evaluate, trace_evidence

class ComputerUseRegressionGateTests(unittest.TestCase):
    def records(self, code, output='TextEdit target screenshot image'):
        return [{'timestamp':'2026-09-22T10:00:00Z','type':'response_item','payload':{'type':'function_call','call_id':'native-1','name':'js','namespace':'mcp__cua_repl','arguments':code}},
                {'timestamp':'2026-09-22T10:00:01Z','type':'response_item','payload':{'type':'function_call_output','call_id':'native-1','output':[{'type':'input_text','text':output},{'type':'input_image','image_url':'data:image/png;base64,test'},{'type':'input_image','image_url':'data:image/png;base64,test'}]}}]
    def evidence(self, code, output='target'):
        return trace_evidence(self.records(code, output), '2026-09-22T09:00:00Z')
    def test_saved_file_alone_cannot_pass(self):
        e=trace_evidence([], '2026-09-22T09:00:00Z')
        self.assertEqual(evaluate(e,'expected','expected')['status'],'fail')
    def test_real_native_trace_and_effect_pass(self):
        e=self.evidence('let app=await cua.getApp("TextEdit"); await app.getScreenshot(); await app.paste("expected"); await app.pressKey("super+s");')
        self.assertEqual(evaluate(e,'expected\n','expected')['status'],'pass')
    def test_shell_fallback_fails_even_if_file_matches(self):
        e=self.evidence('await cua.getApp("TextEdit"); await app.getScreenshot(); await app.paste("expected"); await app.pressKey("super+s"); await tools.exec_command({cmd:"write"});')
        self.assertEqual(evaluate(e,'expected','expected')['status'],'fail')
    def test_permission_failure_is_blocked_not_skipped(self):
        e=self.evidence('await cua.getApp("TextEdit");', 'Computer Use server error -10005: timeoutReached')
        self.assertEqual(evaluate(e,'unchanged','expected')['status'],'blocked')
    def test_prompt_text_does_not_count_as_tool_execution(self):
        records=[{'timestamp':'2026-09-22T10:00:00Z','type':'response_item','payload':{'type':'message','role':'user','content':[{'type':'input_text','text':'cua.getApp(); app.getScreenshot(); app.paste(); app.pressKey("super+s")'}]}}]
        self.assertFalse(trace_evidence(records,'2026-09-22T09:00:00Z')['nativeToolCalled'])
    def test_missing_screenshots_fail(self):
        e=self.evidence('await cua.getApp("TextEdit"); await app.getScreenshot(); await app.paste("expected"); await app.pressKey("super+s");')
        e['screenshotsReturned']=0
        self.assertEqual(evaluate(e,'expected','expected')['status'],'fail')
    def test_plain_node_repl_does_not_count_as_native(self):
        records=self.records('cua.getApp("TextEdit")')
        records[0]['payload']['namespace']='mcp__node_repl'
        self.assertFalse(trace_evidence(records,'2026-09-22T09:00:00Z')['nativeToolCalled'])
    def test_calls_before_phase_do_not_count(self):
        self.assertEqual(trace_evidence(self.records('cua.getApp("TextEdit")'),'2026-09-22T11:00:00Z')['toolCalls'],0)

if __name__=='__main__':unittest.main()
